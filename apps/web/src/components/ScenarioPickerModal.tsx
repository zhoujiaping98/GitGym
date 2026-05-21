import { useEffect, useId, useRef } from "react";
import type { KeyboardEvent as ReactKeyboardEvent } from "react";

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
  const bodyId = useId();
  const errorId = useId();
  const listboxId = useId();
  const selectedOptionId =
    selectedScenarioId === null ? undefined : `${listboxId}-option-${selectedScenarioId}`;
  const dialogRef = useRef<HTMLElement | null>(null);
  const previouslyFocusedElementRef = useRef<HTMLElement | null>(null);
  const optionRefs = useRef(new Map<number, HTMLButtonElement>());

  function getFocusableElements() {
    if (!dialogRef.current) {
      return [];
    }

    return Array.from(
      dialogRef.current.querySelectorAll<HTMLElement>(
        [
          'button:not([disabled])',
          '[href]',
          'input:not([disabled])',
          'select:not([disabled])',
          'textarea:not([disabled])',
          '[tabindex]:not([tabindex="-1"])',
        ].join(", "),
      ),
    ).filter(
      (element) =>
        !element.hasAttribute("disabled") &&
        element.getAttribute("aria-hidden") !== "true",
    );
  }

  function focusScenarioAtIndex(index: number) {
    const boundedIndex = Math.max(0, Math.min(index, scenarios.length - 1));
    const scenario = scenarios[boundedIndex];

    if (!scenario) {
      return;
    }

    optionRefs.current.get(scenario.id)?.focus();
    onSelect(scenario.id);
  }

  function handleOptionKeyDown(
    event: ReactKeyboardEvent<HTMLButtonElement>,
    index: number,
  ) {
    switch (event.key) {
      case "ArrowDown":
      case "ArrowRight":
        event.preventDefault();
        focusScenarioAtIndex(index + 1);
        return;
      case "ArrowUp":
      case "ArrowLeft":
        event.preventDefault();
        focusScenarioAtIndex(index - 1);
        return;
      case "Home":
        event.preventDefault();
        focusScenarioAtIndex(0);
        return;
      case "End":
        event.preventDefault();
        focusScenarioAtIndex(scenarios.length - 1);
        return;
      default:
        return;
    }
  }

  useEffect(() => {
    if (open) {
      previouslyFocusedElementRef.current =
        document.activeElement instanceof HTMLElement ? document.activeElement : null;
      window.requestAnimationFrame(() => {
        if (!dialogRef.current) {
          return;
        }

        const focusTarget = dialogRef.current.querySelector<HTMLElement>(
          '[role="option"][tabindex="0"]',
        );
        (focusTarget ?? dialogRef.current).focus();
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
      if (event.key === "Escape") {
        if (!pending) {
          event.preventDefault();
          onClose();
        }
        return;
      }

      if (event.key !== "Tab") {
        return;
      }

      const focusableElements = getFocusableElements();

      if (focusableElements.length === 0) {
        event.preventDefault();
        dialogRef.current?.focus();
        return;
      }

      const firstFocusableElement = focusableElements[0];
      const lastFocusableElement = focusableElements[focusableElements.length - 1];
      const activeElement =
        document.activeElement instanceof HTMLElement ? document.activeElement : null;

      if (event.shiftKey) {
        if (
          activeElement === firstFocusableElement ||
          activeElement === dialogRef.current
        ) {
          event.preventDefault();
          lastFocusableElement?.focus();
        }
        return;
      }

      if (activeElement === lastFocusableElement) {
        event.preventDefault();
        firstFocusableElement?.focus();
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
      <div className="scenario-picker-viewport">
        <section
          aria-describedby={error ? `${bodyId} ${errorId}` : bodyId}
          aria-labelledby="scenario-picker-title"
          aria-modal="true"
          className="scenario-picker-modal"
          ref={dialogRef}
          role="dialog"
          tabIndex={-1}
        >
          <header>
            <span className="preview-label">Scenario picker</span>
            <h2 id="scenario-picker-title">{title}</h2>
            <p id={bodyId}>{body}</p>
          </header>
          <div className="scenario-picker-list-shell">
            <div
              aria-activedescendant={selectedOptionId}
              aria-label="Practice scenarios"
              className="scenario-picker-list"
              id={listboxId}
              role="listbox"
            >
              {scenarios.map((scenario, index) => {
                const isSelected = selectedScenarioId === scenario.id;
                const isFirstOption = index === 0;
                const isTabbable =
                  selectedScenarioId === null ? isFirstOption : isSelected;

                return (
                  <button
                    key={scenario.id}
                    id={`${listboxId}-option-${scenario.id}`}
                    aria-selected={isSelected}
                    aria-setsize={scenarios.length}
                    aria-posinset={index + 1}
                    className="scenario-picker-option"
                    data-selected={isSelected}
                    onClick={() => onSelect(scenario.id)}
                    onKeyDown={(event) => handleOptionKeyDown(event, index)}
                    ref={(element) => {
                      if (element) {
                        optionRefs.current.set(scenario.id, element);
                        return;
                      }

                      optionRefs.current.delete(scenario.id);
                    }}
                    role="option"
                    tabIndex={isTabbable ? 0 : -1}
                    type="button"
                  >
                    <strong>{scenario.name}</strong>
                    <span>{scenario.key}</span>
                    <span>Template: {scenario.templateName}</span>
                  </button>
                );
              })}
            </div>
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
    </div>
  );
}
