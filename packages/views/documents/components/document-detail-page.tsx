"use client";

import { useCallback, useState } from "react";
import { AlertCircle, ArrowLeft, FileText } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  documentListOptions,
  documentDetailOptions,
  documentRevisionsOptions,
  documentRevisionOptions,
} from "@multica/core/documents";
import {
  useUpdateDocumentContent,
  usePinDocument,
  useArchiveDocument,
  useRestoreDocumentRevision,
} from "@multica/core/documents";
import type { WorkspaceDocument } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@multica/ui/components/ui/tabs";
import { toast } from "sonner";
import { useNavigation } from "../../navigation";
import { PageHeader } from "../../layout/page-header";
import { DocumentTreeSidebar } from "./document-tree-sidebar";
import { DocumentHeader } from "./document-header";
import { DocumentEditor } from "./document-editor";
import { DocumentRevisionHistory } from "./document-revision-history";
import { DocumentDiffViewer } from "./document-diff-viewer";
import { useT } from "../../i18n";

interface DocumentDetailPageProps {
  documentId: string;
}

export function DocumentDetailPage({ documentId }: DocumentDetailPageProps) {
  const { t } = useT("documents");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();

  const { data: documents = [] } = useQuery(documentListOptions(wsId));
  const {
    data: document,
    isLoading,
    error,
  } = useQuery(documentDetailOptions(wsId, documentId));
  const { data: revisions = [] } = useQuery(
    documentRevisionsOptions(wsId, documentId),
  );

  const updateContent = useUpdateDocumentContent();
  const pinDocument = usePinDocument();
  const archiveDocument = useArchiveDocument();
  const restoreRevision = useRestoreDocumentRevision();

  const [activeTab, setActiveTab] = useState("editor");
  const [diffRevisionNumber, setDiffRevisionNumber] = useState<number | null>(null);

  // Fetch the revision for diff viewing
  const { data: diffRevision } = useQuery(
    documentRevisionOptions(wsId, documentId, diffRevisionNumber ?? 0),
  );

  const handleSave = useCallback(
    (content: string) => {
      if (!document) return;
      updateContent.mutate(
        {
          path: document.path,
          content,
          change_summary: "Edit via web UI",
        },
        {
          onError: (err) => {
            toast.error(
              err instanceof Error
                ? err.message
                : t(($) => $.detail.toast_save_failed),
            );
          },
        },
      );
    },
    [document, updateContent, t],
  );

  const handleTogglePin = () => {
    if (!document) return;
    pinDocument.mutate(
      { id: document.id, pinned: !document.pinned },
      {
        onSuccess: () => {
          toast.success(
            document.pinned
              ? t(($) => $.detail.toast_unpinned)
              : t(($) => $.detail.toast_pinned),
          );
        },
      },
    );
  };

  const handleArchive = () => {
    if (!document) return;
    archiveDocument.mutate(
      { id: document.id },
      {
        onSuccess: () => {
          toast.success(t(($) => $.detail.toast_archived));
          navigation.push(paths.documents());
        },
      },
    );
  };

  const handleRestore = (revisionNumber: number) => {
    restoreRevision.mutate(
      { id: documentId, revision_number: revisionNumber },
      {
        onSuccess: () => {
          toast.success(t(($) => $.detail.toast_restored));
          setActiveTab("editor");
        },
      },
    );
  };

  const handleViewRevision = (revisionNumber: number) => {
    setDiffRevisionNumber(revisionNumber);
    setActiveTab("diff");
  };

  const handleSelect = (doc: WorkspaceDocument) => {
    navigation.push(paths.documentDetail(doc.id));
  };

  // --- Loading ---
  if (isLoading) {
    return (
      <div className="flex flex-1 min-h-0 flex-col">
        <PageHeader className="px-5">
          <div className="flex items-center gap-2">
            <FileText className="h-4 w-4 text-muted-foreground" />
            <h1 className="text-sm font-medium">{t(($) => $.page.title)}</h1>
          </div>
        </PageHeader>
        <div className="flex flex-1 min-h-0">
          <div className="w-64 border-r p-3 space-y-2">
            <Skeleton className="h-7 w-full rounded-md" />
            <Skeleton className="h-5 w-3/4 rounded-md" />
          </div>
          <div className="flex-1 p-6 space-y-3">
            <Skeleton className="h-10 w-full rounded-md" />
            <Skeleton className="h-64 w-full rounded-md" />
          </div>
        </div>
      </div>
    );
  }

  // --- Not found ---
  if (error || !document) {
    return (
      <div className="flex flex-1 min-h-0 flex-col">
        <PageHeader className="px-5">
          <div className="flex items-center gap-2">
            <FileText className="h-4 w-4 text-muted-foreground" />
            <h1 className="text-sm font-medium">{t(($) => $.page.title)}</h1>
          </div>
        </PageHeader>
        <div className="flex flex-1 flex-col items-center justify-center gap-3 px-6 py-16 text-center">
          <AlertCircle className="h-8 w-8 text-destructive" />
          <p className="text-sm font-medium">
            {t(($) => $.detail.not_found.title)}
          </p>
          <p className="text-xs text-muted-foreground">
            {t(($) => $.detail.not_found.fallback)}
          </p>
          <Button
            variant="outline"
            size="sm"
            onClick={() => navigation.push(paths.documents())}
          >
            <ArrowLeft className="mr-1 h-3 w-3" />
            {t(($) => $.detail.not_found.back)}
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-1 min-h-0 flex-col">
      <PageHeader className="px-5">
        <div className="flex items-center gap-2">
          <Button
            variant="ghost"
            size="sm"
            className="h-6 px-1.5"
            onClick={() => navigation.push(paths.documents())}
          >
            <ArrowLeft className="h-3.5 w-3.5" />
          </Button>
          <FileText className="h-4 w-4 text-muted-foreground" />
          <h1 className="text-sm font-medium">{t(($) => $.page.title)}</h1>
        </div>
      </PageHeader>

      <div className="flex flex-1 min-h-0">
        {/* Tree sidebar */}
        <DocumentTreeSidebar
          documents={documents}
          selectedId={documentId}
          onSelect={handleSelect}
          className="w-64 shrink-0"
        />

        {/* Main content */}
        <div className="flex flex-1 min-h-0 flex-col">
          <DocumentHeader
            document={document}
            onTogglePin={handleTogglePin}
            onArchive={handleArchive}
          />

          <Tabs
            value={activeTab}
            onValueChange={setActiveTab}
            className="flex flex-1 min-h-0 flex-col"
          >
            <div className="shrink-0 border-b px-4">
              <TabsList className="h-9 bg-transparent p-0">
                <TabsTrigger
                  value="editor"
                  className="rounded-none border-b-2 border-transparent px-3 py-1.5 text-xs data-[state=active]:border-primary data-[state=active]:bg-transparent"
                >
                  Editor
                </TabsTrigger>
                <TabsTrigger
                  value="history"
                  className="rounded-none border-b-2 border-transparent px-3 py-1.5 text-xs data-[state=active]:border-primary data-[state=active]:bg-transparent"
                >
                  {t(($) => $.history.title)}
                  {revisions.length > 0 && (
                    <span className="ml-1 text-muted-foreground/70">
                      {revisions.length}
                    </span>
                  )}
                </TabsTrigger>
                {diffRevisionNumber !== null && (
                  <TabsTrigger
                    value="diff"
                    className="rounded-none border-b-2 border-transparent px-3 py-1.5 text-xs data-[state=active]:border-primary data-[state=active]:bg-transparent"
                  >
                    {t(($) => $.diff.title)}
                  </TabsTrigger>
                )}
              </TabsList>
            </div>

            <TabsContent value="editor" className="flex flex-1 min-h-0 mt-0">
              <DocumentEditor
                content={document.content}
                onSave={handleSave}
              />
            </TabsContent>

            <TabsContent value="history" className="flex flex-1 min-h-0 flex-col mt-0">
              <DocumentRevisionHistory
                revisions={revisions}
                onRestore={handleRestore}
                onViewRevision={handleViewRevision}
              />
            </TabsContent>

            {diffRevisionNumber !== null && (
              <TabsContent value="diff" className="flex flex-1 min-h-0 mt-0">
                <DocumentDiffViewer
                  oldContent={diffRevision?.content ?? ""}
                  newContent={document.content}
                  oldLabel={t(($) => $.diff.old_revision, {
                    number: String(diffRevisionNumber),
                  })}
                  newLabel={t(($) => $.diff.new_revision)}
                  onClose={() => {
                    setDiffRevisionNumber(null);
                    setActiveTab("editor");
                  }}
                />
              </TabsContent>
            )}
          </Tabs>
        </div>
      </div>
    </div>
  );
}
