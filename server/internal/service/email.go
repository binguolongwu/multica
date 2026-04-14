package service

import (
	"fmt"
	"os"

	"github.com/resend/resend-go/v2"
)

// EmailService 邮件发送服务，使用 Resend API。
// 功能：验证码邮件、邀请邮件等。
// 降级策略：如果未配置 RESEND_API_KEY，仅在控制台打印（开发环境）。
type EmailService struct {
	client    *resend.Client // Resend API 客户端
	fromEmail string         // 发件人邮箱地址
}

// NewEmailService 创建邮件服务实例。
// 环境变量：
//   - RESEND_API_KEY: Resend API 密钥（未设置时进入开发模式）
//   - RESEND_FROM_EMAIL: 发件人地址（默认 noreply@multica.ai）
func NewEmailService() *EmailService {
	apiKey := os.Getenv("RESEND_API_KEY")
	from := os.Getenv("RESEND_FROM_EMAIL")
	if from == "" {
		from = "noreply@multica.ai"
	}

	var client *resend.Client
	if apiKey != "" {
		client = resend.NewClient(apiKey)
	}

	return &EmailService{
		client:    client,
		fromEmail: from,
	}
}

// SendVerificationCode 发送验证码邮件。
// 使用场景：用户登录/注册时发送一次性验证码。
// 邮件有效期：10 分钟（在邮件内容中说明）。
func (s *EmailService) SendVerificationCode(to, code string) error {
	if s.client == nil {
		fmt.Printf("[DEV] Verification code for %s: %s\n", to, code)
		return nil
	}

	params := &resend.SendEmailRequest{
		From:    s.fromEmail,
		To:      []string{to},
		Subject: "Your Multica verification code",
		Html: fmt.Sprintf(
			`<div style="font-family: sans-serif; max-width: 400px; margin: 0 auto;">
				<h2>Your verification code</h2>
				<p style="font-size: 32px; font-weight: bold; letter-spacing: 8px; margin: 24px 0;">%s</p>
				<p>This code expires in 10 minutes.</p>
				<p style="color: #666; font-size: 14px;">If you didn't request this code, you can safely ignore this email.</p>
			</div>`, code),
	}

	_, err := s.client.Emails.Send(params)
	return err
}
// SendInvitationEmail 发送工作空间邀请邮件。
// 业务逻辑：
//   1. 构建应用 URL（用户登录后可查看待处理邀请）
//   2. 开发模式（无 API Key）仅打印到控制台
//   3. 生产模式通过 Resend 发送 HTML 邮件
// 参数：
//   - to: 被邀请人邮箱
//   - inviterName: 邀请人姓名（用于邮件个性化）
//   - workspaceName: 工作空间名称
func (s *EmailService) SendInvitationEmail(to, inviterName, workspaceName string) error {
	// 构建应用 URL，用户登录后可在侧边栏的工作空间切换器中看到待处理邀请
	appURL := strings.TrimSpace(os.Getenv("FRONTEND_ORIGIN"))
	if appURL == "" {
		appURL = "https://app.multica.ai"
	}

	if s.client == nil {
		fmt.Printf("[DEV] Invitation email to %s: %s invited you to %s — %s\n", to, inviterName, workspaceName, appURL)
		return nil
	}

	params := &resend.SendEmailRequest{
		From:    s.fromEmail,
		To:      []string{to},
		Subject: fmt.Sprintf("%s invited you to %s on Multica", inviterName, workspaceName),
		Html: fmt.Sprintf(
			`<div style="font-family: sans-serif; max-width: 480px; margin: 0 auto;">
				<h2>You're invited to join %s</h2>
				<p><strong>%s</strong> invited you to collaborate in the <strong>%s</strong> workspace on Multica.</p>
				<p style="margin: 24px 0;">
					<a href="%s" style="display: inline-block; padding: 12px 24px; background: #000; color: #fff; text-decoration: none; border-radius: 6px; font-weight: 500;">Open Multica</a>
				</p>
				<p style="color: #666; font-size: 14px;">Log in to accept or decline the invitation.</p>
			</div>`, workspaceName, inviterName, workspaceName, appURL),
	}

	_, err := s.client.Emails.Send(params)
	return err
}
