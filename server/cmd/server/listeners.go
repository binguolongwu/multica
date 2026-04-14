package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// registerListeners 注册事件总线监听器，将内部事件转换为 WebSocket 广播。
// 事件分发策略：
//   - 个人事件（收件箱、邀请）：通过 SendToUser 仅发送给目标用户
//   - 工作空间事件：广播到整个工作空间房间，所有在线成员收到通知
// 这是实时同步的核心机制，连接后端事件与前端实时更新。
func registerListeners(bus *events.Bus, hub *realtime.Hub) {
	// personalEvents 定义不应广播到整个工作空间的个人事件。
	// 这些事件涉及用户个人隐私（如收件箱通知），只发送给特定用户。
	personalEvents := map[string]bool{
		protocol.EventInboxNew:           true,
		protocol.EventInboxRead:          true,
		protocol.EventInboxArchived:      true,
		protocol.EventInboxBatchRead:     true,
		protocol.EventInboxBatchArchived: true,
	}

	// sendToRecipient 辅助函数：序列化事件并发送给指定用户。
	sendToRecipient := func(hub *realtime.Hub, e events.Event, recipientID string) {
		if recipientID == "" {
			return
		}
		data, err := json.Marshal(map[string]any{"type": e.Type, "payload": e.Payload, "actor_id": e.ActorID})
		if err != nil {
			return
		}
		hub.SendToUser(recipientID, data)
	}

	// inbox:new 事件处理：从嵌套的 item 对象中提取接收者 ID
	// 业务场景：新通知到达时，仅通知该用户（而非整个工作空间）
	bus.Subscribe(protocol.EventInboxNew, func(e events.Event) {
		payload, ok := e.Payload.(map[string]any)
		if !ok {
			return
		}
		item, ok := payload["item"].(map[string]any)
		if !ok {
			return
		}
		recipientID, _ := item["recipient_id"].(string)
		sendToRecipient(hub, e, recipientID)
	})

	// inbox:read 等系列事件处理：从顶层 payload 提取接收者 ID
	// 这些事件表示用户操作了自己的收件箱，只需通知该用户更新侧边栏未读数。
	for _, eventType := range []string{
		protocol.EventInboxRead, protocol.EventInboxArchived,
		protocol.EventInboxBatchRead, protocol.EventInboxBatchArchived,
	} {
		bus.Subscribe(eventType, func(e events.Event) {
			payload, ok := e.Payload.(map[string]any)
			if !ok {
				return
			}
			recipientID, _ := payload["recipient_id"].(string)
			sendToRecipient(hub, e, recipientID)
		})
	}

	// invitation:created 事件处理：发送给被邀请人，使其实时看到邀请
	// 实现细节：
	//   1. 尝试从 payload 获取 InvitationResponse 结构
	//   2. 回退到 map 解析（处理不同序列化方式）
	//   3. 如果 invitee_user_id 存在（已注册用户），发送个人通知
	// 注意：未注册用户（invitee_user_id 为空）不会收到实时通知，依赖邮件。
	bus.Subscribe(protocol.EventInvitationCreated, func(e events.Event) {
		payload, ok := e.Payload.(map[string]any)
		if !ok {
			return
		}
		inv, ok := payload["invitation"].(handler.InvitationResponse)
		if !ok {
			// Fallback for map encoding.
			if invMap, ok := payload["invitation"].(map[string]any); ok {
				if uid, _ := invMap["invitee_user_id"].(*string); uid != nil && *uid != "" {
					data, err := json.Marshal(map[string]any{"type": e.Type, "payload": e.Payload, "actor_id": e.ActorID})
					if err != nil {
						return
					}
					hub.SendToUser(*uid, data)
				}
			}
			return
		}
		if inv.InviteeUserID != nil && *inv.InviteeUserID != "" {
			data, err := json.Marshal(map[string]any{"type": e.Type, "payload": e.Payload, "actor_id": e.ActorID})
			if err != nil {
				return
			}
			hub.SendToUser(*inv.InviteeUserID, data)
		}
	})

	// invitation:revoked 事件处理：通知被邀请人邀请已被撤销
	// 使其待处理邀请列表实时更新，移除已撤销的邀请。
	bus.Subscribe(protocol.EventInvitationRevoked, func(e events.Event) {
		payload, ok := e.Payload.(map[string]any)
		if !ok {
			return
		}
		uid, _ := payload["invitee_user_id"].(*string)
		if uid != nil && *uid != "" {
			sendToRecipient(hub, e, *uid)
		}
	})

	// member:added 事件处理：通知新成员其已被添加到工作空间
	// 业务场景：用户接受邀请后，需要立即在侧边栏看到新工作空间。
	// excludeWorkspace 参数：避免重复发送（SubscribeAll 已通过 BroadcastToWorkspace 广播）
	bus.Subscribe(protocol.EventMemberAdded, func(e events.Event) {
		payload, ok := e.Payload.(map[string]any)
		if !ok {
			return
		}
		var userID string
		switch m := payload["member"].(type) {
		case handler.MemberWithUserResponse:
			userID = m.UserID
		case map[string]any:
			userID, _ = m["user_id"].(string)
		default:
			slog.Warn("member:added: unexpected member payload type", "type", fmt.Sprintf("%T", payload["member"]))
		}
		if userID == "" {
			return
		}
		data, err := json.Marshal(map[string]any{"type": e.Type, "payload": e.Payload, "actor_id": e.ActorID})
		if err != nil {
			return
		}
		hub.SendToUser(userID, data, e.WorkspaceID)
	})

	// SubscribeAll handles workspace-broadcast for non-personal events.
	bus.SubscribeAll(func(e events.Event) {
		// Skip personal events — they are handled by type-specific listeners above.
		if personalEvents[e.Type] {
			return
		}

		msg := map[string]any{
			"type":     e.Type,
			"payload":  e.Payload,
			"actor_id": e.ActorID,
		}
		data, err := json.Marshal(msg)
		if err != nil {
			slog.Error("failed to marshal event", "event_type", e.Type, "error", err)
			return
		}
		if e.WorkspaceID != "" {
			hub.BroadcastToWorkspace(e.WorkspaceID, data)
		} else if strings.HasPrefix(e.Type, "daemon:") {
			hub.Broadcast(data)
		}
		// Otherwise drop — no global broadcast for non-daemon events without a workspace.
	})
}
