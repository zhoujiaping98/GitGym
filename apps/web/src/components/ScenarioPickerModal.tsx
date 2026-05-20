import { useEffect, useId, useRef } from "react";

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
  const titleId = useId();
  const bodyId = useId();
  const errorId = useId();
  const listboxId = useId();
  const selectedOptionId =
    selectedScenarioId === null ? undefined : `${listboxId}-option-${selectedScenarioId}`;
  const dialogRef = useRef<HTMLElement | null>(null);
  const previouslyFocusedElementRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (open) {
      previouslyFocusedElementRef.current =
        document.activeElement instanceof HTMLElement ? document.activeElement : null;
      window.requestAnimationFrame(() => {
        if (!dialogRef.current) {
          return;
        }

        const selectedOption = dialogRef.current.querySelector<HTMLElement>(
          '[role="option"][aria-selected="true"]',
        );
        (selectedOption ?? dialogRef.current).focus();
      });
      return;
    }

    previouslyFocusedElementRef.current?.focus();
    previouslyFocusedElementRef.current = null;
  }, [open]);

  useEffect(() => {
    if (!open) {
      return;
    }

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape" && !pending) {
        event.preventDefault();
        onClose();
      }
    }

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onClose, open, pending]);

  if (!open) {
    return null;
  }

  return (
    <div className="scenario-picker-backdrop" role="presentation">
      <section
        aria-describedby={error ? `${bodyId} ${errorId}` : bodyId}
        aria-labelledby={titleId}
        aria-modal="true"
        className="scenario-picker-modal"
        ref={dialogRef}
        role="dialog"
        tabIndex={-1}
      >
        <header>
          <span className="preview-label">Scenario picker</span>
          <h2 id={titleId}>{title}</h2>
          <p id={bodyId}>{body}</p>
        </header>
        <div
          aria-activedescendant={selectedOptionId}
          aria-label="Practice scenarios"
          className="scenario-picker-list"
          id={listboxId}
          role="listbox"
        >
          {scenarios.map((scenario, index) => (
            <button
              key={scenario.id}
              id={`${listboxId}-option-${scenario.id}`}
              aria-selected={selectedScenarioId === scenario.id}
              aria-setsize={scenarios.length}
              aria-posinset={index + 1}
              className="scenario-picker-option"
              data-selected={selectedScenarioId === scenario.id}
              onClick={() => onSelect(scenario.id)}
              role="option"
              tabIndex={selectedScenarioId === scenario.id ? 0 : -1}
              type="button"
            >
              <strong>{scenario.name}</strong>
              <span>{scenario.key}</span>
              <span>Template: {scenario.templateName}</span>
            </button>
          ))}
        </div>
        {error ? (
          <div
            aria-atomic="true"
            aria-live="assertive"
            className="session-state-detail"
            id={errorId}
            role="alert"
          >
            {error}
          </div>
        ) : null}
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
