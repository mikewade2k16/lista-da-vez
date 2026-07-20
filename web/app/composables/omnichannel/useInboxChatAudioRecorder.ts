import { computed, ref } from "vue";

export function useInboxChatAudioRecorder(options: {
  getHasActiveConversation: () => boolean;
  onPickAttachment: (payload: { file: File; mode: "voice"; durationSeconds?: number | null }) => void;
  onAutoSend: () => void;
}) {
  const isRecording = ref(false);
  const recordingElapsedMs = ref(0);
  const recordingError = ref("");
  const recordingStartedAt = ref<number | null>(null);
  const recordingWaveLevels = ref<number[]>(Array.from({ length: 42 }, () => 0.12));

  let recordingTicker: number | null = null;
  let mediaRecorder: MediaRecorder | null = null;
  let mediaStream: MediaStream | null = null;
  let recorderChunks: Blob[] = [];
  let discardRecordingOnStop = false;
  let shouldAutoSendOnRecordingStop = false;
  let recordingAudioContext: AudioContext | null = null;
  let recordingAnalyser: AnalyserNode | null = null;
  let recordingSourceNode: MediaStreamAudioSourceNode | null = null;
  let recordingWaveRaf: number | null = null;

  const recordingElapsedLabel = computed(() => {
    const totalSeconds = Math.floor(recordingElapsedMs.value / 1000);
    const minutes = String(Math.floor(totalSeconds / 60)).padStart(2, "0");
    const seconds = String(totalSeconds % 60).padStart(2, "0");
    return `${minutes}:${seconds}`;
  });

  function clearRecordingTicker() {
    if (recordingTicker !== null) {
      window.clearInterval(recordingTicker);
      recordingTicker = null;
    }
  }

  function clearRecordingWaveRaf() {
    if (recordingWaveRaf !== null) {
      window.cancelAnimationFrame(recordingWaveRaf);
      recordingWaveRaf = null;
    }
  }

  function resetRecordingWaveLevels() {
    recordingWaveLevels.value = Array.from({ length: 42 }, () => 0.12);
  }

  function stopRecordingWaveCapture() {
    clearRecordingWaveRaf();

    if (recordingSourceNode) {
      recordingSourceNode.disconnect();
      recordingSourceNode = null;
    }

    if (recordingAnalyser) {
      recordingAnalyser.disconnect();
      recordingAnalyser = null;
    }

    if (recordingAudioContext) {
      void recordingAudioContext.close();
      recordingAudioContext = null;
    }
  }

  function startRecordingWaveCapture(stream: MediaStream) {
    if (!import.meta.client) {
      return;
    }

    const AudioContextCtor = window.AudioContext || (window as Window & { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
    if (!AudioContextCtor) {
      return;
    }

    stopRecordingWaveCapture();
    resetRecordingWaveLevels();

    recordingAudioContext = new AudioContextCtor();
    recordingSourceNode = recordingAudioContext.createMediaStreamSource(stream);
    recordingAnalyser = recordingAudioContext.createAnalyser();
    recordingAnalyser.fftSize = 256;
    recordingAnalyser.smoothingTimeConstant = 0.7;
    recordingSourceNode.connect(recordingAnalyser);

    const buffer = new Uint8Array(recordingAnalyser.fftSize);

    const tick = () => {
      if (!recordingAnalyser) {
        return;
      }

      recordingAnalyser.getByteTimeDomainData(buffer);
      let sumSquares = 0;
      for (let index = 0; index < buffer.length; index += 1) {
        const normalized = (buffer[index] - 128) / 128;
        sumSquares += normalized * normalized;
      }

      const rms = Math.sqrt(sumSquares / buffer.length);
      const level = Math.max(0.08, Math.min(1, rms * 6));
      recordingWaveLevels.value = [...recordingWaveLevels.value.slice(1), level];
      recordingWaveRaf = window.requestAnimationFrame(tick);
    };

    recordingWaveRaf = window.requestAnimationFrame(tick);
  }

  function stopRecordingStream() {
    if (!mediaStream) {
      return;
    }

    for (const track of mediaStream.getTracks()) {
      track.stop();
    }

    mediaStream = null;
  }

  function startRecordingTicker() {
    clearRecordingTicker();
    recordingTicker = window.setInterval(() => {
      if (!recordingStartedAt.value) {
        recordingElapsedMs.value = 0;
        return;
      }

      recordingElapsedMs.value = Date.now() - recordingStartedAt.value;
    }, 250);
  }

  async function startRecording() {
    if (!import.meta.client || isRecording.value || !options.getHasActiveConversation()) {
      return;
    }

    const mediaDevices = navigator.mediaDevices;
    if (!mediaDevices?.getUserMedia || typeof MediaRecorder === "undefined") {
      recordingError.value = "Gravacao nao suportada neste navegador.";
      return;
    }

    try {
      recordingError.value = "";
      recorderChunks = [];
      discardRecordingOnStop = false;
      shouldAutoSendOnRecordingStop = false;

      mediaStream = await mediaDevices.getUserMedia({ audio: true });
      startRecordingWaveCapture(mediaStream);

      const candidates = [
        "audio/ogg;codecs=opus",
        "audio/webm;codecs=opus",
        "audio/ogg",
        "audio/webm",
        "audio/mpeg"
      ];
      const mimeType = candidates.find((entry) =>
        typeof MediaRecorder.isTypeSupported === "function" ? MediaRecorder.isTypeSupported(entry) : true
      );

      mediaRecorder = mimeType
        ? new MediaRecorder(mediaStream, { mimeType })
        : new MediaRecorder(mediaStream);

      mediaRecorder.ondataavailable = (event: BlobEvent) => {
        if (event.data && event.data.size > 0) {
          recorderChunks.push(event.data);
        }
      };

      mediaRecorder.onerror = () => {
        recordingError.value = "Falha ao gravar audio.";
        stopRecording(true);
      };

      mediaRecorder.onstop = () => {
        const chunks = recorderChunks;
        const shouldDiscard = discardRecordingOnStop;
        const shouldAutoSend = shouldAutoSendOnRecordingStop;
        const elapsedAtStopMs = recordingElapsedMs.value;
        recorderChunks = [];
        discardRecordingOnStop = false;
        shouldAutoSendOnRecordingStop = false;
        const recorderMimeType = mediaRecorder?.mimeType || chunks[0]?.type || "audio/webm";

        isRecording.value = false;
        recordingStartedAt.value = null;
        recordingElapsedMs.value = 0;
        clearRecordingTicker();
        stopRecordingWaveCapture();
        resetRecordingWaveLevels();
        stopRecordingStream();

        if (!chunks.length || shouldDiscard) {
          mediaRecorder = null;
          return;
        }

        const extension = recorderMimeType.includes("ogg")
          ? "ogg"
          : recorderMimeType.includes("mpeg")
            ? "mp3"
            : "webm";

        const timestamp = new Date().toISOString().replace(/[:.]/g, "-");
        const file = new File([new Blob(chunks, { type: recorderMimeType })], `audio-${timestamp}.${extension}`, {
          type: recorderMimeType
        });
        const durationSeconds = Math.max(1, Math.round(elapsedAtStopMs / 1000));

        options.onPickAttachment({
          file,
          mode: "voice",
          durationSeconds
        });

        if (shouldAutoSend) {
          options.onAutoSend();
        }

        mediaRecorder = null;
      };

      mediaRecorder.start(250);
      isRecording.value = true;
      recordingStartedAt.value = Date.now();
      recordingElapsedMs.value = 0;
      startRecordingTicker();
    } catch {
      recordingError.value = "Nao foi possivel acessar o microfone.";
      stopRecording(true);
    }
  }

  function stopRecording(silent = false, autoSend = false) {
    discardRecordingOnStop = silent;
    shouldAutoSendOnRecordingStop = !silent && autoSend;

    if (!isRecording.value && !mediaRecorder) {
      clearRecordingTicker();
      stopRecordingWaveCapture();
      resetRecordingWaveLevels();
      stopRecordingStream();
      discardRecordingOnStop = false;
      shouldAutoSendOnRecordingStop = false;
      return;
    }

    try {
      if (mediaRecorder && mediaRecorder.state !== "inactive") {
        mediaRecorder.stop();
      } else {
        isRecording.value = false;
        recordingStartedAt.value = null;
        recordingElapsedMs.value = 0;
        clearRecordingTicker();
        stopRecordingWaveCapture();
        resetRecordingWaveLevels();
        stopRecordingStream();
        mediaRecorder = null;
        discardRecordingOnStop = false;
        shouldAutoSendOnRecordingStop = false;
      }
    } catch {
      if (!silent) {
        recordingError.value = "Falha ao finalizar a gravacao.";
      }
      isRecording.value = false;
      recordingStartedAt.value = null;
      recordingElapsedMs.value = 0;
      clearRecordingTicker();
      stopRecordingWaveCapture();
      resetRecordingWaveLevels();
      stopRecordingStream();
      mediaRecorder = null;
      discardRecordingOnStop = false;
      shouldAutoSendOnRecordingStop = false;
    }
  }

  function toggleRecording() {
    if (isRecording.value) {
      stopRecording();
      return;
    }

    void startRecording();
  }

  function cancelRecording() {
    stopRecording(true);
  }

  function sendRecordedAudio() {
    stopRecording(false, true);
  }

  function disposeRecording() {
    stopRecording(true);
    stopRecordingWaveCapture();
  }

  return {
    isRecording,
    recordingElapsedMs,
    recordingElapsedLabel,
    recordingError,
    recordingWaveLevels,
    startRecording,
    stopRecording,
    toggleRecording,
    cancelRecording,
    sendRecordedAudio,
    disposeRecording
  };
}
