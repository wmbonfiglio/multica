// @vitest-environment jsdom

import { describe, it, expect, vi, beforeEach } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, fireEvent } from "@testing-library/react";
import type { MemberWithUser, RuntimeDevice } from "@multica/core/types";
import { I18nProvider } from "@multica/core/i18n/react";
import enCommon from "../../locales/en/common.json";
import enAgents from "../../locales/en/agents.json";

const TEST_RESOURCES = { en: { common: enCommon, agents: enAgents } };

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

// ModelDropdown talks to the api; the create dialog only needs it as a
// stand-in here, so swap it out.
vi.mock("./model-dropdown", () => ({
  ModelDropdown: () => null,
}));

// Provider logos don't matter for these assertions but they pull in SVGs.
vi.mock("../../runtimes/components/provider-logo", () => ({
  ProviderLogo: () => null,
}));

// Avatars hit the api for member metadata.
vi.mock("../../common/actor-avatar", () => ({
  ActorAvatar: () => null,
}));

vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}));

import { CreateAgentDialog } from "./create-agent-dialog";

const ME = "user-me";
const OTHER = "user-other";

const members: MemberWithUser[] = [
  {
    id: "m-me",
    user_id: ME,
    workspace_id: "ws-1",
    role: "member",
    name: "Me",
    email: "me@example.com",
    avatar_url: null,
    created_at: "2026-01-01T00:00:00Z",
  },
  {
    id: "m-other",
    user_id: OTHER,
    workspace_id: "ws-1",
    role: "member",
    name: "Other",
    email: "other@example.com",
    avatar_url: null,
    created_at: "2026-01-01T00:00:00Z",
  },
];

function makeRuntime(overrides: Partial<RuntimeDevice>): RuntimeDevice {
  return {
    id: "rt",
    workspace_id: "ws-1",
    daemon_id: null,
    name: "Test Runtime",
    runtime_mode: "local",
    provider: "claude",
    launch_header: "",
    status: "online",
    device_info: "host.local",
    metadata: {},
    owner_id: ME,
    visibility: "private",
    timezone: "UTC",
    last_seen_at: "2026-04-27T11:59:50Z",
    created_at: "2026-04-01T00:00:00Z",
    updated_at: "2026-04-01T00:00:00Z",
    ...overrides,
  };
}

function renderDialog(runtimes: RuntimeDevice[]) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const onCreate = vi.fn().mockResolvedValue(undefined);
  const onClose = vi.fn();
  render(
    <I18nProvider locale="en" resources={TEST_RESOURCES}>
      <QueryClientProvider client={queryClient}>
        <CreateAgentDialog
          runtimes={runtimes}
          members={members}
          currentUserId={ME}
          onClose={onClose}
          onCreate={onCreate}
        />
      </QueryClientProvider>
    </I18nProvider>,
  );
  return { onCreate, onClose };
}

describe("CreateAgentDialog runtime visibility gate", () => {
  beforeEach(() => vi.clearAllMocks());

  it("disables another member's private runtime in the picker", () => {
    const mine = makeRuntime({ id: "rt-mine", name: "My Runtime", owner_id: ME, visibility: "private" });
    const othersPrivate = makeRuntime({
      id: "rt-others-private",
      name: "Others Private",
      owner_id: OTHER,
      visibility: "private",
    });
    renderDialog([mine, othersPrivate]);

    // Flip to "All" so other-owned runtimes show.
    fireEvent.click(screen.getByText("All"));
    // Open the picker.
    fireEvent.click(
      screen.getByText("My Runtime", { selector: "span.truncate" }),
    );

    const disabledRow = screen
      .getByText("Others Private")
      .closest("button") as HTMLButtonElement;
    expect(disabledRow).not.toBeNull();
    expect(disabledRow.disabled).toBe(true);
    expect(disabledRow.title).toMatch(/Private runtime/i);
  });

  it("lets a plain member pick another member's public runtime", () => {
    const mine = makeRuntime({ id: "rt-mine", name: "My Runtime", owner_id: ME, visibility: "private" });
    const othersPublic = makeRuntime({
      id: "rt-others-public",
      name: "Others Public",
      owner_id: OTHER,
      visibility: "public",
    });
    renderDialog([mine, othersPublic]);

    fireEvent.click(screen.getByText("All"));
    fireEvent.click(
      screen.getByText("My Runtime", { selector: "span.truncate" }),
    );

    const publicRow = screen
      .getByText("Others Public")
      .closest("button") as HTMLButtonElement;
    expect(publicRow).not.toBeNull();
    expect(publicRow.disabled).toBe(false);
  });

  it("defaults the selected runtime to a usable one, not a locked private", () => {
    const othersPrivate = makeRuntime({
      id: "rt-others-private",
      name: "Others Private",
      owner_id: OTHER,
      visibility: "private",
    });
    const mine = makeRuntime({
      id: "rt-mine",
      name: "My Runtime",
      owner_id: ME,
      visibility: "private",
    });
    renderDialog([othersPrivate, mine]);

    // The trigger label shows the selected runtime name. The picker must
    // not seed with the other-owned private runtime even if it sorted
    // first in the input list.
    expect(screen.queryByText("Others Private", { selector: "span.truncate" })).toBeNull();
    expect(screen.getByText("My Runtime", { selector: "span.truncate" })).toBeInTheDocument();
  });
});
