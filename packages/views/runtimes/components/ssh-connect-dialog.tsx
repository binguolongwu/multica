"use client";

import { useId, useState } from "react";
import type { FormEvent } from "react";
import { Loader2, Server } from "lucide-react";
import { toast } from "sonner";
import { api } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { ProviderLogo } from "./provider-logo";

const AVAILABLE_RUNTIMES: { id: string; label: string }[] = [
  { id: "claude", label: "Claude (Anthropic)" },
  { id: "codebuddy", label: "CodeBuddy (Tencent)" },
  { id: "codex", label: "Codex (OpenAI)" },
  { id: "copilot", label: "GitHub Copilot" },
  { id: "opencode", label: "OpenCode" },
  { id: "openclaw", label: "OpenClaw" },
  { id: "hermes", label: "Hermes (NousResearch)" },
  { id: "gemini", label: "Gemini CLI (Google)" },
  { id: "pi", label: "Pi (pi.dev)" },
  { id: "cursor", label: "Cursor" },
  { id: "kimi", label: "Kimi (Moonshot)" },
  { id: "kiro", label: "Kiro CLI" },
  { id: "antigravity", label: "Antigravity (Google)" },
];

export function SSHConnectDialog({ onClose }: { onClose: () => void }) {
  const wsId = useWorkspaceId();
  const idPrefix = `ssh-connect-${useId().replace(/:/g, "")}`;
  const formId = `${idPrefix}-form`;
  const [host, setHost] = useState("");
  const [port, setPort] = useState("22");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [selectedRuntimes, setSelectedRuntimes] = useState<string[]>(["claude"]);
  const [connecting, setConnecting] = useState(false);

  const toggleRuntime = (id: string) => {
    setSelectedRuntimes((prev) =>
      prev.includes(id) ? prev.filter((r) => r !== id) : [...prev, id],
    );
  };

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!host.trim()) {
      toast.error("请输入SSH服务器地址");
      return;
    }
    if (!username.trim()) {
      toast.error("请输入用户名");
      return;
    }
    if (selectedRuntimes.length === 0) {
      toast.error("请至少选择一个运行时");
      return;
    }

    setConnecting(true);
    try {
      const result = await api.sshConnectRuntime(wsId, {
        host: host.trim(),
        port: port.trim() || "22",
        username: username.trim(),
        password,
        runtimes: selectedRuntimes,
      });
      if (result.ok === "true") {
        toast.success("远程服务器连接成功，正在安装并启动运行时...");
        onClose();
      } else {
        toast.error(result.error || "连接失败");
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "连接失败");
    } finally {
      setConnecting(false);
    }
  };

  return (
    <Dialog open onOpenChange={() => onClose()}>
      <DialogContent style={{ width: "60vw", maxWidth: "60vw", maxHeight: "80vh" }} className="overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Server className="h-4 w-4" />
            添加SSH服务器
          </DialogTitle>
          <DialogDescription>
            通过SSH连接到远程服务器，选择要安装的运行时，自动安装并注册到当前工作区。
          </DialogDescription>
        </DialogHeader>

        <form id={formId} onSubmit={handleSubmit} className="space-y-4">
          <div className="grid grid-cols-3 gap-3">
            <div className="col-span-2">
              <Label htmlFor={`${idPrefix}-host`}>SSH服务器地址 *</Label>
              <Input
                id={`${idPrefix}-host`}
                value={host}
                onChange={(e) => setHost(e.target.value)}
                placeholder="192.168.1.100 或 your-server.com"
                className="mt-1"
              />
            </div>
            <div>
              <Label htmlFor={`${idPrefix}-port`}>端口</Label>
              <Input
                id={`${idPrefix}-port`}
                value={port}
                onChange={(e) => setPort(e.target.value)}
                placeholder="22"
                className="mt-1"
              />
            </div>
          </div>

          <div>
            <Label htmlFor={`${idPrefix}-username`}>用户名 *</Label>
            <Input
              id={`${idPrefix}-username`}
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="root"
              className="mt-1"
            />
          </div>

          <div>
            <Label htmlFor={`${idPrefix}-password`}>密码</Label>
            <Input
              id={`${idPrefix}-password`}
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="输入SSH密码"
              className="mt-1"
            />
          </div>

          <div>
            <Label className="text-xs font-medium mb-2 block">
              选择要安装的运行时 * (已选 {selectedRuntimes.length} 个)
            </Label>
            <div className="grid grid-cols-2 gap-1 max-h-48 overflow-y-auto border rounded-lg p-2">
              {AVAILABLE_RUNTIMES.map((rt) => (
                <label
                  key={rt.id}
                  className={`flex items-center gap-2 px-2 py-1.5 rounded text-sm cursor-pointer hover:bg-muted transition-colors ${
                    selectedRuntimes.includes(rt.id) ? "bg-primary/10" : ""
                  }`}
                >
                  <Checkbox
                    checked={selectedRuntimes.includes(rt.id)}
                    onCheckedChange={() => toggleRuntime(rt.id)}
                  />
                  <ProviderLogo provider={rt.id} className="h-4 w-4 shrink-0" />
                  <span className="truncate">{rt.label}</span>
                </label>
              ))}
            </div>
          </div>
        </form>

        <DialogFooter>
          <Button variant="outline" size="sm" onClick={onClose} disabled={connecting}>
            取消
          </Button>
          <Button
            size="sm"
            type="submit"
            form={formId}
            disabled={connecting || !host.trim() || !username.trim() || selectedRuntimes.length === 0}
          >
            {connecting && <Loader2 className="h-3 w-3 animate-spin mr-1" />}
            {connecting ? "安装中..." : "登录并安装"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
