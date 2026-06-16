package lifecycle

import "path/filepath"

type PipelinePaths struct {
	BronzeTelemetryCSV   string
	SilverTelemetryCSV   string
	QualityReportJSON    string
	GoldFeaturesCSV      string
	GoldEventsCSV        string
	MonitoringReportJSON string
}

type PipelineResult struct {
	Paths             PipelinePaths
	QualityReport     QualityReport
	MonitoringSummary MonitoringSummary
}

func Paths(cfg Config) PipelinePaths {
	return PipelinePaths{
		BronzeTelemetryCSV:   filepath.Join(cfg.DataDir, "bronze", "vehicle_telemetry.csv"),
		SilverTelemetryCSV:   filepath.Join(cfg.DataDir, "silver", "vehicle_telemetry_validated.csv"),
		QualityReportJSON:    filepath.Join(cfg.DataDir, "silver", "data_quality_report.json"),
		GoldFeaturesCSV:      filepath.Join(cfg.DataDir, "gold", "vehicle_features.csv"),
		GoldEventsCSV:        filepath.Join(cfg.DataDir, "gold", "events.csv"),
		MonitoringReportJSON: filepath.Join(cfg.DataDir, "gold", "monitoring_summary.json"),
	}
}

func RunAll(cfg Config) (PipelineResult, error) {
	paths := Paths(cfg)

	records := GenerateTelemetry(cfg.Rows, cfg.Vehicles, cfg.Seed)
	if err := WriteTelemetryCSV(paths.BronzeTelemetryCSV, records); err != nil {
		return PipelineResult{}, err
	}

	validation := ValidateTelemetry(records)
	if err := WriteTelemetryCSV(paths.SilverTelemetryCSV, validation.ValidRecords); err != nil {
		return PipelineResult{}, err
	}
	if err := WriteJSON(paths.QualityReportJSON, validation.Report); err != nil {
		return PipelineResult{}, err
	}

	features := BuildFeatures(validation.ValidRecords, cfg.Thresholds)
	if err := WriteFeatureCSV(paths.GoldFeaturesCSV, features); err != nil {
		return PipelineResult{}, err
	}
	if err := WriteFeatureCSV(paths.GoldEventsCSV, EventRecords(features)); err != nil {
		return PipelineResult{}, err
	}

	monitoring := SummarizeFeatures(features)
	if err := WriteJSON(paths.MonitoringReportJSON, monitoring); err != nil {
		return PipelineResult{}, err
	}

	return PipelineResult{
		Paths:             paths,
		QualityReport:     validation.Report,
		MonitoringSummary: monitoring,
	}, nil
}

func RunGenerate(cfg Config) (PipelinePaths, error) {
	paths := Paths(cfg)
	records := GenerateTelemetry(cfg.Rows, cfg.Vehicles, cfg.Seed)
	return paths, WriteTelemetryCSV(paths.BronzeTelemetryCSV, records)
}

func RunValidate(cfg Config) (QualityReport, error) {
	paths := Paths(cfg)
	records, err := ReadTelemetryCSV(paths.BronzeTelemetryCSV)
	if err != nil {
		return QualityReport{}, err
	}
	validation := ValidateTelemetry(records)
	if err := WriteTelemetryCSV(paths.SilverTelemetryCSV, validation.ValidRecords); err != nil {
		return QualityReport{}, err
	}
	if err := WriteJSON(paths.QualityReportJSON, validation.Report); err != nil {
		return QualityReport{}, err
	}
	return validation.Report, nil
}

func RunFeatures(cfg Config) (MonitoringSummary, error) {
	paths := Paths(cfg)
	records, err := ReadTelemetryCSV(paths.SilverTelemetryCSV)
	if err != nil {
		return MonitoringSummary{}, err
	}
	features := BuildFeatures(records, cfg.Thresholds)
	if err := WriteFeatureCSV(paths.GoldFeaturesCSV, features); err != nil {
		return MonitoringSummary{}, err
	}
	if err := WriteFeatureCSV(paths.GoldEventsCSV, EventRecords(features)); err != nil {
		return MonitoringSummary{}, err
	}
	summary := SummarizeFeatures(features)
	if err := WriteJSON(paths.MonitoringReportJSON, summary); err != nil {
		return MonitoringSummary{}, err
	}
	return summary, nil
}
