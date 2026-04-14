import { create } from "zustand";
import type { User, StorageAdapter } from "../types";
import type { ApiClient } from "../api/client";

export interface AuthStoreOptions {
  api: ApiClient;
  storage: StorageAdapter;
  onLogin?: () => void;
  onLogout?: () => void;
  /** 当为 true 时，使用 HttpOnly Cookie 而非 localStorage 存储认证令牌 */
  cookieAuth?: boolean;
}

export interface AuthState {
  user: User | null;
  isLoading: boolean;

  initialize: () => Promise<void>;
  sendCode: (email: string) => Promise<void>;
  verifyCode: (email: string, code: string) => Promise<User>;
  loginWithGoogle: (code: string, redirectUri: string) => Promise<User>;
  loginWithToken: (token: string) => Promise<User>;
  logout: () => void;
  setUser: (user: User) => void;
}

export function createAuthStore(options: AuthStoreOptions) {
  const { api, storage, onLogin, onLogout, cookieAuth } = options;

  return create<AuthState>((set) => ({
    user: null,
    isLoading: true,

    // 初始化认证状态
    initialize: async () => {
      if (cookieAuth) {
        // Cookie 模式：HttpOnly Cookie 会自动发送
        // 尝试获取当前用户——如果 Cookie 存在服务器会接受
        try {
          const user = await api.getMe();
          set({ user, isLoading: false });
        } catch {
          set({ user: null, isLoading: false });
        }
        return;
      }

      // 令牌模式：从 localStorage 读取（Electron / 旧版）
      const token = storage.getItem("multica_token");
      if (!token) {
        set({ isLoading: false });
        return;
      }

      api.setToken(token);

      try {
        const user = await api.getMe();
        set({ user, isLoading: false });
      } catch {
        api.setToken(null);
        api.setWorkspaceId(null);
        storage.removeItem("multica_token");
        set({ user: null, isLoading: false });
      }
    },

    sendCode: async (email: string) => {
      await api.sendCode(email);
    },

    verifyCode: async (email: string, code: string) => {
      const { token, user } = await api.verifyCode(email, code);
      if (!cookieAuth) {
        // 令牌模式：为 Electron / 旧版持久化存储
        storage.setItem("multica_token", token);
        api.setToken(token);
      }
      onLogin?.();
      set({ user });
      return user;
    },

    loginWithGoogle: async (code: string, redirectUri: string) => {
      const { token, user } = await api.googleLogin(code, redirectUri);
      if (!cookieAuth) {
        storage.setItem("multica_token", token);
        api.setToken(token);
      }
      onLogin?.();
      set({ user });
      return user;
    },

    loginWithToken: async (token: string) => {
      storage.setItem("multica_token", token);
      api.setToken(token);
      const user = await api.getMe();
      onLogin?.();
      set({ user, isLoading: false });
      return user;
    },

    logout: () => {
      if (cookieAuth) {
        // Clear server-side HttpOnly cookie.
        api.logout().catch(() => {});
      }
      storage.removeItem("multica_token");
      api.setToken(null);
      api.setWorkspaceId(null);
      onLogout?.();
      set({ user: null });
    },

    setUser: (user: User) => {
      set({ user });
    },
  }));
}
