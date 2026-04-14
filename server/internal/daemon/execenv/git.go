package execenv

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// detectGitRepo 检测指定目录是否在 git 仓库中（普通仓库或裸仓库）。
// 返回 git 根目录路径和布尔值表示是否找到。
// 业务逻辑：用于确定代码执行环境的 git 上下文，为 Agent 任务准备正确的代码库。
func detectGitRepo(dir string) (string, bool) {
	// Try regular repo first.
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	if out, err := cmd.Output(); err == nil {
		return strings.TrimSpace(string(out)), true
	}

	// Try bare repo: git-dir is "." for bare repos when -C points at the repo.
	cmd = exec.Command("git", "-C", dir, "rev-parse", "--is-bare-repository")
	if out, err := cmd.Output(); err == nil && strings.TrimSpace(string(out)) == "true" {
		return dir, true
	}

	return "", false
}

// fetchOrigin 执行 `git fetch origin` 确保本地仓库有最新的远程引用。
// 业务逻辑：在 Agent 开始工作前拉取最新代码，确保操作基于最新版本。
func fetchOrigin(gitRoot string) error {
	cmd := exec.Command("git", "-C", gitRoot, "fetch", "origin")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch origin: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// getRemoteDefaultBranch 返回远程仓库默认分支的名称，格式为 "origin/<分支名>"。
// 回退策略：先尝试 origin/main，然后是 origin/master，最后使用 HEAD。
// 业务逻辑：确定代码库的主分支，用于创建 Agent 的工作分支。
func getRemoteDefaultBranch(gitRoot string) string {
	// Try symbolic-ref of origin/HEAD (set by `git clone` or `git remote set-head`).
	cmd := exec.Command("git", "-C", gitRoot, "symbolic-ref", "refs/remotes/origin/HEAD")
	if out, err := cmd.Output(); err == nil {
		ref := strings.TrimSpace(string(out))
		// ref looks like "refs/remotes/origin/main" — return "origin/main".
		if strings.HasPrefix(ref, "refs/remotes/") {
			return strings.TrimPrefix(ref, "refs/remotes/")
		}
		return ref
	}

	// Fallback: check if origin/main exists.
	cmd = exec.Command("git", "-C", gitRoot, "rev-parse", "--verify", "origin/main")
	if err := cmd.Run(); err == nil {
		return "origin/main"
	}

	// Fallback: check if origin/master exists.
	cmd = exec.Command("git", "-C", gitRoot, "rev-parse", "--verify", "origin/master")
	if err := cmd.Run(); err == nil {
		return "origin/master"
	}

	return "HEAD"
}

// setupGitWorktree 在指定路径创建 git worktree（工作树）和新分支。
// 业务逻辑：为每个 Agent 任务创建隔离的工作目录，避免相互干扰。
// 如果分支名冲突，会自动追加时间戳重试。
func setupGitWorktree(gitRoot, worktreePath, branchName, baseRef string) error {
	// Remove the workdir created by caller — git worktree add needs to create it.
	if err := os.Remove(worktreePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove placeholder workdir: %w", err)
	}

	err := runGitWorktreeAdd(gitRoot, worktreePath, branchName, baseRef)
	if err != nil && strings.Contains(err.Error(), "already exists") {
		// Branch name collision: append timestamp and retry once.
		branchName = fmt.Sprintf("%s-%d", branchName, time.Now().Unix())
		err = runGitWorktreeAdd(gitRoot, worktreePath, branchName, baseRef)
	}
	return err
}

// runGitWorktreeAdd 执行 git worktree add 命令创建独立工作目录。
// 参数：gitRoot-原仓库路径, worktreePath-新工作目录, branchName-新分支名, baseRef-基于的提交/分支
func runGitWorktreeAdd(gitRoot, worktreePath, branchName, baseRef string) error {
	cmd := exec.Command("git", "-C", gitRoot, "worktree", "add", "-b", branchName, worktreePath, baseRef)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree add: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// removeGitWorktree 删除 git worktree 和其对应的分支。
// 业务逻辑：Agent 任务完成后清理资源，采用尽力而为策略（错误仅记录不中断）。
// 这是资源管理的关键函数，防止磁盘空间泄漏。
func removeGitWorktree(gitRoot, worktreePath, branchName string, logger *slog.Logger) {
	// Remove the worktree.
	cmd := exec.Command("git", "-C", gitRoot, "worktree", "remove", "--force", worktreePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		logger.Warn("execenv: git worktree remove failed", "output", strings.TrimSpace(string(out)), "error", err)
	}

	// Delete the branch (best-effort).
	if branchName != "" {
		cmd = exec.Command("git", "-C", gitRoot, "branch", "-D", branchName)
		if out, err := cmd.CombinedOutput(); err != nil {
			logger.Warn("execenv: git branch delete failed", "branch", branchName, "output", strings.TrimSpace(string(out)), "error", err)
		}
	}
}

// excludeFromGit 将指定模式添加到工作目录的 .git/info/exclude 文件中。
// 业务逻辑：用于将 Agent 生成的临时文件、日志等排除在版本控制之外，
// 避免 Agent 工作时意外提交不必要的文件。
func excludeFromGit(worktreePath, pattern string) error {
	// Resolve the actual git dir for this worktree.
	cmd := exec.Command("git", "-C", worktreePath, "rev-parse", "--git-dir")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("resolve git dir: %w", err)
	}

	gitDir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(worktreePath, gitDir)
	}

	excludePath := filepath.Join(gitDir, "info", "exclude")

	// Ensure the info directory exists.
	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return fmt.Errorf("create info dir: %w", err)
	}

	// Check if pattern is already present.
	existing, _ := os.ReadFile(excludePath)
	if strings.Contains(string(existing), pattern) {
		return nil
	}

	f, err := os.OpenFile(excludePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open exclude file: %w", err)
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "\n%s\n", pattern); err != nil {
		return fmt.Errorf("write exclude pattern: %w", err)
	}
	return nil
}

// repoNameFromURL 从 git 远程 URL 中提取简短的目录名称。
// 例如："https://github.com/org/my-repo.git" → "my-repo"
// 业务逻辑：用于本地缓存目录命名，支持 HTTPS 和 SSH 格式的 URL。
func repoNameFromURL(url string) string {
	// Strip trailing slashes and .git suffix.
	url = strings.TrimRight(url, "/")
	url = strings.TrimSuffix(url, ".git")

	// Take the last path segment.
	if i := strings.LastIndex(url, "/"); i >= 0 {
		url = url[i+1:]
	}
	// Also handle SSH-style "host:org/repo".
	if i := strings.LastIndex(url, ":"); i >= 0 {
		url = url[i+1:]
		if j := strings.LastIndex(url, "/"); j >= 0 {
			url = url[j+1:]
		}
	}

	name := strings.TrimSpace(url)
	if name == "" {
		return "repo"
	}
	return name
}

// shortID 返回 UUID 字符串的前 8 个字符（去掉横线）。
// 业务逻辑：生成人类可读的唯一标识，用于分支名、目录名等场景。
func shortID(uuid string) string {
	s := strings.ReplaceAll(uuid, "-", "")
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

// sanitizeName 将可读字符串转换为 git 分支安全的名称。
// 处理规则：转小写、非字母数字替换为横线、去除首尾横线、截断至30字符、空值默认"agent"。
// 业务逻辑：确保用户输入（如 Agent 名称）可用于创建合法的分支名。
func sanitizeName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = nonAlphanumeric.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 30 {
		s = s[:30]
		s = strings.TrimRight(s, "-")
	}
	if s == "" {
		s = "agent"
	}
	return s
}
