package statsig_go_core

type FeatureGateEvaluationOptions struct {
	DisableExposureLogging bool `json:"disable_exposure_logging"`
}

type DynamicConfigEvaluationOptions struct {
	DisableExposureLogging bool `json:"disable_exposure_logging"`
}

type ExperimentEvaluationOptions struct {
	DisableExposureLogging bool                 `json:"disable_exposure_logging"`
	UserPersistedValues    *UserPersistedValues `json:"user_persisted_values,omitempty"`
	// When a persisted sticky value exists, let a matching console override
	// rule take precedence over it.
	EnforceOverrides bool `json:"enforce_overrides"`
	// When a persisted sticky value exists, re-check targeting and drop the
	// sticky value if the user no longer passes targeting.
	EnforceTargeting bool `json:"enforce_targeting"`
}

type LayerEvaluationOptions struct {
	DisableExposureLogging bool                 `json:"disable_exposure_logging"`
	UserPersistedValues    *UserPersistedValues `json:"user_persisted_values,omitempty"`
	// When a persisted sticky value exists, let a matching console override
	// rule take precedence over it.
	EnforceOverrides bool `json:"enforce_overrides"`
	// When a persisted sticky value exists, re-check targeting and drop the
	// sticky value if the user no longer passes targeting.
	EnforceTargeting bool `json:"enforce_targeting"`
}

type ParameterStoreEvaluationOptions struct {
	DisableExposureLogging bool `json:"disable_exposure_logging"`
}
