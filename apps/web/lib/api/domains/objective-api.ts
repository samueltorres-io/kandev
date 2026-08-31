import { getWebSocketClient } from "@/lib/ws/connection";
import type { TaskObjectiveAssessment, TaskObjectiveRun } from "@/lib/types/objective";

// i18n-exempt: precondition diagnostic for a programmer error; callers branch on
// the error type, never render this message.
const WS_CLIENT_UNAVAILABLE = "WebSocket client not available";

/**
 * Error thrown by a rejected objective-assessment action, carrying the backend's
 * machine-readable `objective_*` code so the surface can show an actionable
 * message instead of a generic failure.
 */
export class ObjectiveRequestError extends Error {
  readonly code: string;

  constructor(message: string, code: string) {
    super(message);
    this.name = "ObjectiveRequestError";
    this.code = code;
  }
}

function requireClient() {
  const client = getWebSocketClient();
  if (!client) throw new Error(WS_CLIENT_UNAVAILABLE);
  return client;
}

const KNOWN_CODES = [
  "objective_agent_unavailable",
  "objective_workspace_unavailable",
  "objective_no_changes",
  "objective_no_objective",
  "objective_unparseable_response",
  "objective_execution_failed",
  "objective_cancelled",
];

function extractCodeFromMessage(message: string): string {
  return KNOWN_CODES.find((code) => message.includes(code)) ?? "";
}

function toObjectiveError(error: unknown): ObjectiveRequestError {
  if (error instanceof ObjectiveRequestError) return error;
  const message = error instanceof Error ? error.message : String(error);
  const details = (error as { details?: Record<string, unknown> } | null)?.details;
  const code = typeof details?.code === "string" ? details.code : extractCodeFromMessage(message);
  return new ObjectiveRequestError(message, code);
}

/** Starts an assessment pass. Resolves with the pending run; inference continues server-side. */
export async function runObjectiveAssessment(params: {
  taskId: string;
  sessionId?: string;
  repositoryId?: string;
  agentProfileId?: string;
}): Promise<TaskObjectiveRun> {
  try {
    const response = await requireClient().request<{ run: TaskObjectiveRun }>(
      "task.objective.run",
      {
        task_id: params.taskId,
        session_id: params.sessionId ?? "",
        repository_id: params.repositoryId ?? "",
        agent_profile_id: params.agentProfileId ?? "",
      },
      20000,
    );
    return response.run;
  } catch (error) {
    throw toObjectiveError(error);
  }
}

/** Cancels an in-flight assessment run. Idempotent for an already-finished run. */
export async function cancelObjectiveAssessment(runId: string): Promise<TaskObjectiveRun> {
  const response = await requireClient().request<{ run: TaskObjectiveRun }>("task.objective.cancel", {
    run_id: runId,
  });
  return response.run;
}

/** Loads a task's assessment run history and latest criteria, to backfill the store on mount. */
export async function getObjectiveAssessment(taskId: string): Promise<TaskObjectiveAssessment> {
  const response = await requireClient().request<TaskObjectiveAssessment>("task.objective.get", {
    task_id: taskId,
  });
  return {
    runs: response?.runs ?? [],
    criteria: response?.criteria ?? [],
    verdict: response?.verdict ?? "",
  };
}

/** Removes a task's assessment runs and criteria. */
export async function clearObjectiveAssessment(taskId: string): Promise<void> {
  await requireClient().request("task.objective.clear", { task_id: taskId });
}
