package app

import (
	"context"

	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/broker"
	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/samples"
)

type ReplayResult struct {
	Report              samples.Report `json:"report"`
	PublishedRaw        int            `json:"published_raw"`
	PublishedNormalized int            `json:"published_normalized"`
}

func ValidateSamples(samplesDir string) (samples.Report, error) {
	return samples.Validate(samplesDir)
}

func ReplaySamples(ctx context.Context, samplesDir string, publisher broker.Publisher) (ReplayResult, error) {
	report, err := samples.Validate(samplesDir)
	if err != nil {
		return ReplayResult{}, err
	}
	rawRows, err := samples.LoadRaw(samplesDir)
	if err != nil {
		return ReplayResult{}, err
	}
	normalizedRows, err := samples.LoadNormalized(samplesDir)
	if err != nil {
		return ReplayResult{}, err
	}
	cexRows, _ := samples.CEXNormalized(normalizedRows)

	result := ReplayResult{Report: report}
	for _, row := range rawRows {
		if _, err := publisher.PublishRaw(ctx, row); err != nil {
			return ReplayResult{}, err
		}
		result.PublishedRaw++
	}
	for _, row := range cexRows {
		if _, err := publisher.PublishNormalized(ctx, row); err != nil {
			return ReplayResult{}, err
		}
		result.PublishedNormalized++
	}
	return result, nil
}
