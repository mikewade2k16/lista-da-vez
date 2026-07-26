package transcriptions

import "testing"

func TestMergeTranscriptTextRemovesOverlappingWords(t *testing.T) {
	t.Parallel()

	got := mergeTranscriptText(
		"A cliente procura um vestido azul para a formatura",
		"para a formatura e prefere um modelo sem brilho.",
	)
	want := "A cliente procura um vestido azul para a formatura e prefere um modelo sem brilho."
	if got != want {
		t.Fatalf("mergeTranscriptText() = %q, want %q", got, want)
	}
}

func TestMergeTranscriptTextKeepsNonOverlappingSpeech(t *testing.T) {
	t.Parallel()

	got := mergeTranscriptText("Primeira parte.", "Segunda parte.")
	want := "Primeira parte. Segunda parte."
	if got != want {
		t.Fatalf("mergeTranscriptText() = %q, want %q", got, want)
	}
}

func TestMergeTranscriptTextDoesNotRepeatContainedWindow(t *testing.T) {
	t.Parallel()

	got := mergeTranscriptText("A cliente pediu o vestido azul.", "o vestido azul.")
	want := "A cliente pediu o vestido azul."
	if got != want {
		t.Fatalf("mergeTranscriptText() = %q, want %q", got, want)
	}
}
