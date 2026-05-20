package service

import (
	"context"
	"fmt"
)

type PracticeCatalog interface {
	ListTemplates(ctx context.Context) ([]PracticeTemplate, error)
	ListScenarios(ctx context.Context) ([]PracticeScenario, error)
	TemplateByID(ctx context.Context, templateID uint64) (PracticeTemplate, error)
	ScenarioByID(ctx context.Context, scenarioID uint64) (PracticeScenario, error)
}

type staticPracticeCatalog struct {
	templates []PracticeTemplate
	scenarios []PracticeScenario
}

func NewFallbackPracticeCatalog() PracticeCatalog {
	return NewStaticPracticeCatalog(
		[]PracticeTemplate{
			{ID: 1, Key: "standard", Name: "Standard"},
		},
		[]PracticeScenario{
			{ID: 1, Key: "sandbox-standard", Name: "Standard Sandbox", TemplateID: 1},
		},
	)
}

func NewStaticPracticeCatalog(templates []PracticeTemplate, scenarios []PracticeScenario) PracticeCatalog {
	templateCopy := append([]PracticeTemplate(nil), templates...)
	scenarioCopy := append([]PracticeScenario(nil), scenarios...)

	return staticPracticeCatalog{
		templates: templateCopy,
		scenarios: scenarioCopy,
	}
}

func (c staticPracticeCatalog) ListTemplates(context.Context) ([]PracticeTemplate, error) {
	return append([]PracticeTemplate(nil), c.templates...), nil
}

func (c staticPracticeCatalog) ListScenarios(context.Context) ([]PracticeScenario, error) {
	return append([]PracticeScenario(nil), c.scenarios...), nil
}

func (c staticPracticeCatalog) TemplateByID(_ context.Context, templateID uint64) (PracticeTemplate, error) {
	for _, template := range c.templates {
		if template.ID == templateID {
			return template, nil
		}
	}

	return PracticeTemplate{}, fmt.Errorf("%w: %d", ErrUnknownPracticeTemplate, templateID)
}

func (c staticPracticeCatalog) ScenarioByID(_ context.Context, scenarioID uint64) (PracticeScenario, error) {
	for _, scenario := range c.scenarios {
		if scenario.ID == scenarioID {
			return scenario, nil
		}
	}

	return PracticeScenario{}, fmt.Errorf("%w: %d", ErrUnknownPracticeScenario, scenarioID)
}
