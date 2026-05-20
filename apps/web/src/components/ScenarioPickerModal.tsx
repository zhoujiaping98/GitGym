type ScenarioPickerOption = {
  id: number;
  name: string;
  key: string;
  templateName: string;
};

type ScenarioPickerModalProps = {
  open: boolean;
  title: string;
  body: string;
  scenarios: ScenarioPickerOption[];
  selectedScenarioId: number | null;
  pending: boolean;
  error: string | null;
  confirmLabel?: string;
  onSelect: (scenarioId: number) => void;
  onConfirm: () => void;
  onClose: () => void;
};

export function ScenarioPickerModal({
  open,
  title,
  body,
  scenarios,
  selectedScenarioId,
  pending,
  error,
  confirmLabel = "Start Session",
  onSelect,
  onConfirm,
  onClose,
}: ScenarioPickerModalProps) {
  if (!open) {
    return null;
  }

  return (
    <div className="scenario-picker-backdrop" role="presentation">
      <section
        aria-labelledby="scenario-picker-title"
        aria-modal="true"
        className="scenario-picker-modal"
        role="dialog"
      >
        <header>
          <span className="preview-label">Scenario picker</span>
          <h2 id="scenario-picker-title">{title}</h2>
          <p>{body}</p>
        </header>
        <div className="scenario-picker-list" role="listbox" aria-label="Practice scenarios">
          {scenarios.map((scenario) => (
            <button
              key={scenario.id}
              aria-selected={selectedScenarioId === scenario.id}
              className="scenario-picker-option"
              data-selected={selectedScenarioId === scenario.id}
              onClick={() => onSelect(scenario.id)}
              type="button"
            >
              <strong>{scenario.name}</strong>
              <span>{scenario.key}</span>
              <span>Template: {scenario.templateName}</span>
            </button>
          ))}
        </div>
        {error ? <div className="session-state-detail">{error}</div> : null}
        <div className="scenario-picker-actions">
          <button className="top-bar-button" disabled={pending} onClick={onClose} type="button">
            Cancel
          </button>
          <button
            className="primary-button"
            disabled={pending || selectedScenarioId === null}
            onClick={onConfirm}
            type="button"
          >
            {pending ? "Starting..." : confirmLabel}
          </button>
        </div>
      </section>
    </div>
  );
}
