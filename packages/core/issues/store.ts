"use client";
// 问题状态管理（客户端状态）
// 功能：跟踪当前活跃问题 ID（用于侧边栏详情展示）

import { create } from "zustand";

// 问题客户端状态接口
interface IssueClientState {
  activeIssueId: string | null;  // 当前活跃问题 ID
  setActiveIssue: (id: string | null) => void;  // 设置活跃问题
}

// 问题状态存储
export const useIssueStore = create<IssueClientState>((set) => ({
  activeIssueId: null,
  setActiveIssue: (id) => set({ activeIssueId: id }),
}));
