"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";

interface DocumentEditorProps {
  content: string;
  onSave: (content: string) => void;
  disabled?: boolean;
  className?: string;
}

/**
 * Markdown textarea with debounced autosave (900ms).
 * Ported from the paperclip IssueDocumentsSection pattern.
 */
export function DocumentEditor({
  content,
  onSave,
  disabled,
  className,
}: DocumentEditorProps) {
  const { t } = useT("documents");
  const [draft, setDraft] = useState(content);
  const [saveStatus, setSaveStatus] = useState<"saved" | "saving" | "unsaved">("saved");
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const onSaveRef = useRef(onSave);
  onSaveRef.current = onSave;

  // Reset draft when content changes externally (e.g. revision restore)
  useEffect(() => {
    setDraft(content);
    setSaveStatus("saved");
  }, [content]);

  const scheduleSave = useCallback(
    (value: string) => {
      if (timerRef.current) clearTimeout(timerRef.current);
      setSaveStatus("unsaved");
      timerRef.current = setTimeout(() => {
        setSaveStatus("saving");
        onSaveRef.current(value);
        // Assume save succeeds — the mutation's onSuccess will reset
        setSaveStatus("saved");
      }, 900);
    },
    [],
  );

  // Cleanup timer on unmount
  useEffect(() => {
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, []);

  const handleChange = (value: string) => {
    setDraft(value);
    scheduleSave(value);
  };

  return (
    <div className={cn("flex flex-1 flex-col", className)}>
      <Textarea
        value={draft}
        onChange={(e) => handleChange(e.target.value)}
        disabled={disabled}
        placeholder={t(($) => $.editor.placeholder)}
        className="flex-1 resize-none rounded-none border-0 font-mono text-sm leading-relaxed focus-visible:ring-0"
      />
      <div className="flex h-7 shrink-0 items-center justify-end border-t px-3">
        <span
          className={cn(
            "text-[10px]",
            saveStatus === "unsaved"
              ? "text-warning"
              : saveStatus === "saving"
                ? "text-muted-foreground"
                : "text-muted-foreground/50",
          )}
        >
          {saveStatus === "unsaved" && t(($) => $.editor.unsaved)}
          {saveStatus === "saving" && t(($) => $.editor.saving)}
          {saveStatus === "saved" && t(($) => $.editor.saved)}
        </span>
      </div>
    </div>
  );
}
