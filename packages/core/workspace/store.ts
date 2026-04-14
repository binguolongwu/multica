// 工作空间状态管理
// 职责：管理当前选中的工作空间（UI 状态）
// 注意：工作空间列表是服务器状态，存储在 React Query 中

import { create } from "zustand";
import type { Workspace, StorageAdapter } from "../types";
import type { ApiClient } from "../api/client";
import { createLogger } from "../logger";
import { setCurrentWorkspaceId, rehydrateAllWorkspaceStores } from "../platform/workspace-storage";

const logger = createLogger("workspace-store");

interface WorkspaceStoreOptions {
  storage?: StorageAdapter;
}

interface WorkspaceState {
  workspace: Workspace | null;
}

// 工作空间操作接口
interface WorkspaceActions {
  /**
   * 从列表中选择工作空间并设为当前
   * 列表本身不存储在此处——它存在于 React Query 中
   */
  hydrateWorkspace: (
    wsList: Workspace[],
    preferredWorkspaceId?: string | null,
  ) => Workspace | null;
  /** 切换到指定工作空间。调用者提供完整对象（来自 React Query） */
  switchWorkspace: (ws: Workspace) => void;
  /** 就地更新当前工作空间数据（例如重命名后） */
  updateWorkspace: (ws: Workspace) => void;
  /** 清除当前工作空间 */
  clearWorkspace: () => void;
}

export type WorkspaceStore = WorkspaceState & WorkspaceActions;

export function createWorkspaceStore(api: ApiClient, options?: WorkspaceStoreOptions) {
  const storage = options?.storage;

  return create<WorkspaceStore>((set) => ({
    // 仅当前选中的工作空间（UI 状态）
    // 工作空间列表是服务器状态，存在于 React Query 中
    workspace: null,

    hydrateWorkspace: (wsList, preferredWorkspaceId) => {
      const nextWorkspace =
        (preferredWorkspaceId
          ? wsList.find((item) => item.id === preferredWorkspaceId)
          : null) ??
        wsList[0] ??
        null;

      if (!nextWorkspace) {
        api.setWorkspaceId(null);
        setCurrentWorkspaceId(null);
        rehydrateAllWorkspaceStores();
        storage?.removeItem("multica_workspace_id");
        set({ workspace: null });
        return null;
      }

      api.setWorkspaceId(nextWorkspace.id);
      setCurrentWorkspaceId(nextWorkspace.id);
      rehydrateAllWorkspaceStores();
      storage?.setItem("multica_workspace_id", nextWorkspace.id);
      set({ workspace: nextWorkspace });
      logger.debug("hydrate workspace", nextWorkspace.name, nextWorkspace.id);

      return nextWorkspace;
    },

    switchWorkspace: (ws) => {
      logger.info("switching to", ws.id);
      api.setWorkspaceId(ws.id);
      setCurrentWorkspaceId(ws.id);
      rehydrateAllWorkspaceStores();
      storage?.setItem("multica_workspace_id", ws.id);
      set({ workspace: ws });
    },

    updateWorkspace: (ws) => {
      set((state) => ({
        workspace: state.workspace?.id === ws.id ? ws : state.workspace,
      }));
    },

    clearWorkspace: () => {
      api.setWorkspaceId(null);
      setCurrentWorkspaceId(null);
      rehydrateAllWorkspaceStores();
      set({ workspace: null });
    },
  }));
}
