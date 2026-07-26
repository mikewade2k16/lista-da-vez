package core

// experimentalFeaturesKey identifica o documento global de recursos
// experimentais em core.platform_settings.
const experimentalFeaturesKey = "experimental_features"

// ExperimentalFeatures concentra apenas flags de rollout ainda não estáveis.
// Novas flags devem nascer desligadas e ser removidas deste documento quando a
// funcionalidade deixar de ser experimental.
type ExperimentalFeatures struct {
	Version                  int  `json:"version"`
	AttendanceAudioRecording bool `json:"attendanceAudioRecording"`
}

// ExperimentalFeaturesResponse é o contrato autoritativo de leitura e escrita.
type ExperimentalFeaturesResponse struct {
	Features  ExperimentalFeatures `json:"features"`
	UpdatedAt *string              `json:"updatedAt"`
	UpdatedBy *string              `json:"updatedBy"`
}

func defaultExperimentalFeatures() ExperimentalFeatures {
	return ExperimentalFeatures{
		Version:                  1,
		AttendanceAudioRecording: false,
	}
}

func normalizeExperimentalFeatures(features *ExperimentalFeatures) {
	if features.Version < 1 {
		features.Version = 1
	}
}
