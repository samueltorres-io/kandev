"use client";

import { useCallback } from "react";
import { useTranslation } from "react-i18next";
import { useAppStoreApi } from "@/components/state-provider";
import { useToast } from "@/components/toast-provider";
import { appendToQueue } from "@/lib/api/domains/queue-api";
import { formatCriterionAsMarkdown } from "@/lib/objective/format";
import type { TaskObjectiveCriterion } from "@/lib/types/objective";
import type { Message } from "@/lib/types/http";
import { getWebSocketClient } from "@/lib/ws/connection";
import { generateUUID } from "@/lib/utils";

type Params = {
  taskId: string | null | undefined;
  sessionId: string | null | undefined;
};

function isAgentBusy(state: string | undefined): boolean {
  return state === "STARTING" || state === "RUNNING";
}

/**
 * Sends a criterion to the active session as follow-up context. Mirrors
 * `useSendFindingToAgent`: queue while the agent is busy, direct-send otherwise.
 * An assessment is advisory, so this never changes the stored verdict.
 */
export function useSendCriterionToAgent({ taskId, sessionId }: Params) {
  const storeApi = useAppStoreApi();
  const { toast } = useToast();
  const { t } = useTranslation("review");

  return useCallback(
    async (criterion: TaskObjectiveCriterion) => {
      if (!taskId || !sessionId) return;
      const state = storeApi.getState();
      const session = state.taskSessions.items[sessionId] ?? null;
      const content = formatCriterionAsMarkdown(criterion);
      const planMode = state.chatInput.planModeBySessionId[sessionId] ?? false;

      try {
        if (isAgentBusy(session?.state)) {
          await appendToQueue({
            session_id: sessionId,
            task_id: taskId,
            content,
            ...(planMode ? { plan_mode: true } : {}),
          });
          toast({ title: t("review:objectiveCriterionQueuedForAgent"), variant: "success" });
          return;
        }
        const client = getWebSocketClient();
        if (!client) throw new Error("WebSocket client unavailable");
        const created = await client.request<Message | undefined>(
          "message.add",
          {
            task_id: taskId,
            session_id: sessionId,
            client_message_id: generateUUID(),
            content,
            has_review_comments: true,
            ...(planMode ? { plan_mode: true } : {}),
          },
          10000,
        );
        if (created?.id && created.session_id) storeApi.getState().addMessage(created);
        toast({ title: t("review:objectiveCriterionSentToAgent"), variant: "success" });
      } catch (error) {
        toast({
          title: t("review:objectiveCouldNotSendCriterion"),
          description: error instanceof Error ? error.message : t("common:anErrorOccurred"),
          variant: "error",
        });
      }
    },
    [taskId, sessionId, storeApi, t, toast],
  );
}
