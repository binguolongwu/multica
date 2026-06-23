"use client";

import { useState, useMemo, useRef, useEffect } from "react";
import { FileText, FolderOpen, Folder, ChevronRight, ChevronDown, FolderPlus, Trash2 } from "lucide-react";
import type { WikiPage } from "@multica/core/wiki";
import { useT } from "../../i18n";

interface WikiFileTreeProps {
  pages: WikiPage[];
  selectedPath: string | null;
  onSelect: (path: string) => void;
  selectedDir: string;
  onSelectDir: (dir: string) => void;
  onCreateDir: (parentDir: string, name: string) => void;
  onDelete: (targetPath: string, isFile: boolean) => void;
  enabledDirs: string[];
}

interface TreeNode {
  name: string;
  path: string;
  isDir: boolean;
  children: TreeNode[];
}

function buildTree(pages: WikiPage[], enabledDirs: string[], titleMap: Map<string, string>): TreeNode[] {
  // Use empty string as root so all top-level entries (AGENTS.md, IDEA.md, raw/, wiki/) are siblings
  const root: TreeNode = { name: "", path: "", isDir: true, children: [] };
  const pathMap = new Map<string, TreeNode>();
  pathMap.set("", root);

  for (const page of pages) {
    const isGitkeep = page.path.endsWith("/.gitkeep");
    if (page.title) titleMap.set(page.path, page.title);

    const parts = page.path.split("/");
    let currentPath = "";
    for (let i = 0; i < parts.length; i++) {
      const part = parts[i];
      if (!part) continue;
      const isLast = i === parts.length - 1;
      const prevPath = currentPath;
      currentPath = currentPath ? `${currentPath}/${part}` : part;

      if (pathMap.has(currentPath)) continue;

      // Don't create leaf node for .gitkeep — but still propagate parent dirs above
      if (isGitkeep && isLast) continue;

      const node: TreeNode = { name: part, path: currentPath, isDir: !isLast, children: [] };
      pathMap.set(currentPath, node);
      const parent = pathMap.get(prevPath);
      if (parent) parent.children.push(node);
    }
  }

  // Ensure known directories appear even when empty
  const allDirs = enabledDirs.includes("raw") ? enabledDirs : ["raw", ...enabledDirs];
  for (const dir of allDirs) {
    if (pathMap.has(dir)) continue;
    const parts = dir.split("/");
    const name = parts[parts.length - 1] || dir;
    const parentPath = parts.slice(0, -1).join("/");
    const node: TreeNode = { name, path: dir, isDir: true, children: [] };
    pathMap.set(dir, node);
    const parent = pathMap.get(parentPath);
    if (parent) parent.children.push(node);
  }

  const sortNode = (node: TreeNode) => {
    node.children.sort((a, b) => {
      if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
      return a.name.localeCompare(b.name);
    });
    for (const child of node.children) sortNode(child);
  };
  sortNode(root);

  return root.children;
}

function TreeItem({ node, depth, selectedPath, selectedDir, onSelect, onSelectDir, titleMap, creating, newNameInput, onNewNameChange, onCreateSubmit }: {
  node: TreeNode; depth: number; selectedPath: string | null; selectedDir: string; onSelect: (path: string) => void; onSelectDir: (dir: string) => void; titleMap: Map<string, string>;
  creating: boolean; newNameInput: string; onNewNameChange: (v: string) => void; onCreateSubmit: () => void;
}) {
  const [expanded, setExpanded] = useState(true);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (creating && selectedDir === node.path) {
      setExpanded(true);
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  }, [creating, selectedDir, node.path]);

  if (node.isDir) {
    const isSelected = selectedDir === node.path;
    return (
      <div key={node.path}>
        <button
          className={`flex w-full items-center gap-1 rounded px-2 py-1 text-left text-sm hover:bg-accent ${isSelected ? "bg-accent ring-1 ring-inset ring-border" : ""}`}
          style={{ paddingLeft: 12 + depth * 16 }}
          onClick={() => { onSelectDir(node.path); if (!expanded) setExpanded(true); }}
        >
          {(node.children.length > 0 || !expanded) ? (
            <span className="flex h-3.5 w-3.5 items-center justify-center flex-shrink-0" onClick={(e) => { e.stopPropagation(); setExpanded(!expanded); }}>
              {expanded ? <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" /> : <ChevronRight className="h-3.5 w-3.5 text-muted-foreground" />}
            </span>
          ) : (
            <span className="w-3.5 flex-shrink-0" />
          )}
          {expanded ? <FolderOpen className="h-4 w-4 flex-shrink-0 text-muted-foreground" /> : <Folder className="h-4 w-4 flex-shrink-0 text-muted-foreground" />}
          <span className={`truncate text-xs ${isSelected ? "font-medium" : "text-muted-foreground"}`}>{node.name}</span>
        </button>
        {expanded && (
          <>
            {node.children.map((c) => (
              <TreeItem key={c.path} node={c} depth={depth + 1} selectedPath={selectedPath} selectedDir={selectedDir} onSelect={onSelect} onSelectDir={onSelectDir} titleMap={titleMap} creating={creating} newNameInput={newNameInput} onNewNameChange={onNewNameChange} onCreateSubmit={onCreateSubmit} />
            ))}
            {creating && isSelected && (
              <div className="flex items-center gap-1 rounded px-2 py-0.5" style={{ paddingLeft: 12 + (depth + 1) * 16 }}>
                <FileText className="h-4 w-4 flex-shrink-0 text-muted-foreground opacity-50" />
                <input
                  ref={inputRef}
                  className="h-6 flex-1 rounded border bg-background px-1.5 text-xs outline-none focus:border-primary"
                  placeholder="folder name"
                  value={newNameInput}
                  onChange={(e) => onNewNameChange(e.target.value)}
                  onKeyDown={(e) => { if (e.key === "Enter") onCreateSubmit(); if (e.key === "Escape") onNewNameChange(""); }}
                  onBlur={() => { if (!newNameInput.trim()) onNewNameChange(""); }}
                />
              </div>
            )}
          </>
        )}
      </div>
    );
  }
  return (
    <button
      className={`flex w-full items-center gap-1 rounded px-2 py-1 text-left text-sm hover:bg-accent ${selectedPath === node.path ? "bg-accent font-medium" : ""}`}
      style={{ paddingLeft: 12 + depth * 16 }}
      onClick={() => onSelect(node.path)}
    >
      <FileText className="h-4 w-4 flex-shrink-0 text-muted-foreground" />
      <span className="truncate text-xs">{titleMap.get(node.path) || node.name}</span>
    </button>
  );
}

export function WikiFileTree({ pages, selectedPath, selectedDir, onSelect, onSelectDir, onCreateDir, onDelete, enabledDirs }: WikiFileTreeProps) {
  const { t } = useT("layout");
  const { tree, titleMap } = useMemo(() => {
    const m = new Map<string, string>();
    const t = buildTree(pages, enabledDirs, m);
    return { tree: t, titleMap: m };
  }, [pages, enabledDirs]);
  const [newNameInput, setNewNameInput] = useState("");
  const [creating, setCreating] = useState(false);

  const handleNewClick = () => { setCreating(true); setNewNameInput(""); };

  const handleCreate = () => {
    const name = newNameInput.trim();
    if (!name) { setCreating(false); return; }
    onCreateDir(selectedDir, name);
    setNewNameInput("");
    setCreating(false);
  };

  const handleDelete = () => {
    // Prefer deleting the selected file; fall back to selected directory
    if (selectedPath) {
      onDelete(selectedPath, true);
    } else if (selectedDir) {
      onDelete(selectedDir, false);
    }
  };

  if (tree.length === 0) {
    return <div className="p-4 text-center text-xs text-muted-foreground">No pages yet</div>;
  }

  return (
    <div className="flex h-full flex-col overflow-hidden">
      {/* Toolbar */}
      <div className="flex shrink-0 items-center gap-1 border-b px-2 py-1">
        <button
          className="inline-flex h-7 items-center justify-center gap-1 rounded px-2 hover:bg-accent text-xs"
          title={t(($) => $.wiki_page.ingest_dir_new_folder)}
          onClick={handleNewClick}
        >
          <FolderPlus className="h-3.5 w-3.5 text-muted-foreground" />
          <span>{t(($) => $.wiki_page.ingest_dir_new_folder)}</span>
        </button>
        <button
          className="inline-flex h-7 items-center justify-center gap-1 rounded px-2 hover:bg-destructive/10 disabled:opacity-30 text-xs"
          title={t(($) => $.wiki_page.ingest_dir_delete)}
          onClick={handleDelete}
          disabled={!selectedPath && !selectedDir}
        >
          <Trash2 className="h-3.5 w-3.5 text-muted-foreground" />
          <span>{t(($) => $.wiki_page.ingest_dir_delete)}</span>
        </button>
      </div>
      {/* Tree */}
      <div className="flex-1 overflow-y-auto py-1">
        {tree.map((n) => <TreeItem key={n.path} node={n} depth={0} selectedPath={selectedPath} selectedDir={selectedDir} onSelect={onSelect} onSelectDir={onSelectDir} titleMap={titleMap} creating={creating} newNameInput={newNameInput} onNewNameChange={setNewNameInput} onCreateSubmit={handleCreate} />)}
      </div>
    </div>
  );
}
