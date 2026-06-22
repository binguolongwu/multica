"use client";

import { useState, useMemo } from "react";
import { FileText, FolderOpen, Folder, ChevronRight, ChevronDown } from "lucide-react";
import type { WikiPage } from "@multica/core/wiki";

interface WikiFileTreeProps {
  pages: WikiPage[];
  selectedPath: string | null;
  onSelect: (path: string) => void;
}

interface TreeNode {
  name: string;
  path: string;
  isDir: boolean;
  children: TreeNode[];
}

function buildTree(pages: WikiPage[]): TreeNode[] {
  // Use empty string as root so all top-level entries (AGENTS.md, IDEA.md, raw/, wiki/) are siblings
  const root: TreeNode = { name: "", path: "", isDir: true, children: [] };
  const pathMap = new Map<string, TreeNode>();
  pathMap.set("", root);

  for (const page of pages) {
    if (page.path.endsWith("/.gitkeep")) continue;

    const parts = page.path.split("/");
    let currentPath = "";
    for (let i = 0; i < parts.length; i++) {
      const part = parts[i];
      if (!part) continue;
      const isLast = i === parts.length - 1;
      const prevPath = currentPath;
      currentPath = currentPath ? `${currentPath}/${part}` : part;

      if (pathMap.has(currentPath)) continue;

      const node: TreeNode = { name: part, path: currentPath, isDir: !isLast, children: [] };
      pathMap.set(currentPath, node);
      const parent = pathMap.get(prevPath);
      if (parent) parent.children.push(node);
    }
  }

  // Ensure known directories appear even when empty
  const knownDirs = ["raw", "wiki/sources", "wiki/projects", "wiki/entities", "wiki/concepts", "wiki/synthesis", "wiki/learnings"];
  for (const dir of knownDirs) {
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

function TreeItem({ node, depth, selectedPath, onSelect }: {
  node: TreeNode; depth: number; selectedPath: string | null; onSelect: (path: string) => void;
}) {
  const [expanded, setExpanded] = useState(true);
  if (node.isDir) {
    return (
      <div key={node.path}>
        <button
          className="flex w-full items-center gap-1 rounded px-2 py-1 text-left text-sm hover:bg-accent"
          style={{ paddingLeft: 12 + depth * 16 }}
          onClick={() => setExpanded(!expanded)}
        >
          {expanded ? <ChevronDown className="h-3.5 w-3.5 flex-shrink-0 text-muted-foreground" /> : <ChevronRight className="h-3.5 w-3.5 flex-shrink-0 text-muted-foreground" />}
          {expanded ? <FolderOpen className="h-4 w-4 flex-shrink-0 text-muted-foreground" /> : <Folder className="h-4 w-4 flex-shrink-0 text-muted-foreground" />}
          <span className="truncate text-xs font-medium text-muted-foreground">{node.name}</span>
        </button>
        {expanded && node.children.map((c) => (
          <TreeItem key={c.path} node={c} depth={depth + 1} selectedPath={selectedPath} onSelect={onSelect} />
        ))}
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
      <span className="truncate text-xs">{node.name}</span>
    </button>
  );
}

export function WikiFileTree({ pages, selectedPath, onSelect }: WikiFileTreeProps) {
  const tree = useMemo(() => buildTree(pages), [pages]);
  if (tree.length === 0) {
    return <div className="p-4 text-center text-xs text-muted-foreground">No pages yet</div>;
  }
  return (
    <div className="py-1">
      {tree.map((n) => <TreeItem key={n.path} node={n} depth={0} selectedPath={selectedPath} onSelect={onSelect} />)}
    </div>
  );
}
