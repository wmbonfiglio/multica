"use client";

import { type ReactNode, useRef, useState } from "react";
import { ArrowLeft } from "lucide-react";
import { useScrollFade } from "@multica/ui/hooks/use-scroll-fade";
import type { OnboardingStep } from "@multica/core/onboarding";
import { DragStrip } from "@multica/views/platform";
import { StepHeader } from "../components/step-header";
import {
  IconOptionCard,
  IconOtherOptionCard,
} from "../components/icon-option-card";
import { useT } from "../../i18n";

/**
 * One option in the card grid. `slug` is the persisted enum value;
 * `icon` is a React node (lucide icon, brand SVG, or emoji span);
 * `label` is the localized string already resolved by the caller.
 * `isOther` flips this card into a free-text input row.
 */
export interface QuestionOption {
  slug: string;
  icon: ReactNode;
  label: string;
  isOther?: boolean;
}

/**
 * Generic per-question step used by Source / Role / Use case. The
 * parent threads in the question copy, the option list, and the
 * three callbacks (answer / skip / back). Layout is a centered
 * card grid with an editorial heading; below the grid sits a
 * three-button action row — Back, Skip, Continue — keeping the
 * controls visually anchored to the content rather than floating
 * at the page edge. Continue is the only path that advances, so
 * users can change their selection before committing.
 */
export function StepQuestion({
  step,
  number,
  eyebrow,
  question,
  options,
  selectedSlug,
  otherValue,
  onOtherChange,
  otherPlaceholder,
  onAnswer,
  onAdvance,
  onSkip,
  onBack,
}: {
  step: OnboardingStep;
  number: number;
  eyebrow?: string;
  question: string;
  options: readonly QuestionOption[];
  selectedSlug: string | null;
  otherValue: string;
  onOtherChange: (value: string) => void;
  otherPlaceholder: string;
  /** Record the selection in the parent — does NOT advance. */
  onAnswer: (slug: string) => void;
  /** Commit the current selection and move to the next step. */
  onAdvance: () => void;
  onSkip: () => void;
  onBack?: () => void;
}) {
  const { t } = useT("onboarding");
  const [pendingOther, setPendingOther] = useState(false);
  const mainRef = useRef<HTMLElement>(null);
  const fadeStyle = useScrollFade(mainRef);

  const handleSelect = (option: QuestionOption) => {
    if (option.isOther) {
      setPendingOther(true);
      onOtherChange(otherValue);
      onAnswer(option.slug);
      return;
    }
    setPendingOther(false);
    onAnswer(option.slug);
  };

  const selectedOption = options.find((o) => o.slug === selectedSlug) ?? null;
  const otherActive = selectedOption?.isOther || pendingOther;
  const otherFilled = (otherValue ?? "").trim().length > 0;
  // Continue is enabled when:
  //   - a non-Other option is selected, OR
  //   - Other is selected AND the free-text input has content.
  const canContinue = selectedSlug !== null && (!otherActive || otherFilled);

  const confirmAdvance = () => {
    if (canContinue) onAdvance();
  };

  return (
    <div className="animate-onboarding-enter flex h-full min-h-0 flex-col bg-background">
      <DragStrip />
      <header className="flex shrink-0 items-center bg-background px-6 py-3 sm:px-10 md:px-14 lg:px-16">
        <div className="flex-1">
          <StepHeader currentStep={step} />
        </div>
      </header>

      <main
        ref={mainRef}
        style={fadeStyle}
        className="min-h-0 flex-1 overflow-y-auto"
      >
        <div className="mx-auto w-full max-w-[920px] px-6 py-10 sm:px-10 md:px-14 lg:py-16">
          {eyebrow ? (
            <div className="mb-2 text-xs font-medium uppercase tracking-[0.08em] text-muted-foreground">
              {eyebrow}
            </div>
          ) : null}
          <div className="mb-1 font-mono text-xs text-muted-foreground">
            {String(number).padStart(2, "0")}
          </div>
          <h1 className="text-balance font-serif text-[34px] font-medium leading-[1.15] tracking-tight text-foreground">
            {question}
          </h1>

          <fieldset
            role="radiogroup"
            aria-label={question}
            className="mt-10 m-0 grid grid-cols-1 gap-3 p-0 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4"
          >
            {options.map((option) =>
              option.isOther ? (
                <IconOtherOptionCard
                  key={option.slug}
                  icon={option.icon}
                  label={option.label}
                  selected={otherActive}
                  onSelect={() => handleSelect(option)}
                  otherValue={otherValue}
                  onOtherChange={onOtherChange}
                  onConfirm={confirmAdvance}
                  placeholder={otherPlaceholder}
                />
              ) : (
                <IconOptionCard
                  key={option.slug}
                  icon={option.icon}
                  label={option.label}
                  selected={selectedSlug === option.slug && !otherActive}
                  onSelect={() => handleSelect(option)}
                />
              ),
            )}
          </fieldset>

          <div className="mt-8 flex items-center gap-2">
            {onBack ? (
              <button
                type="button"
                onClick={onBack}
                className="flex items-center gap-1.5 rounded-md px-3 py-2 text-sm text-muted-foreground transition-colors hover:bg-accent/40 hover:text-foreground"
              >
                <ArrowLeft className="h-3.5 w-3.5" />
                {t(($) => $.common.back)}
              </button>
            ) : null}
            <button
              type="button"
              onClick={onSkip}
              className="rounded-md px-3 py-2 text-sm text-muted-foreground transition-colors hover:bg-accent/40 hover:text-foreground"
            >
              {t(($) => $.common.skip)}
            </button>
            <button
              type="button"
              onClick={confirmAdvance}
              disabled={!canContinue}
              className="ml-auto rounded-md bg-foreground px-4 py-2 text-sm font-medium text-background transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-30"
            >
              {t(($) => $.common.continue)}
            </button>
          </div>
        </div>
      </main>
    </div>
  );
}
