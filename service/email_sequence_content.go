package service

import (
	"bytes"
	"html/template"
)

// EmailTemplate 单封邮件的主题与正文模板。
type EmailTemplate struct {
	Subject  string
	BodyHTML string
}

// EmailRenderData 渲染邮件时注入的数据。
type EmailRenderData struct {
	SystemName     string
	SignUpLink     string // /sign-up?redirect=/keys + UTM
	QuickstartLink string // /quickstart + UTM
	TopupLink      string // /wallet + UTM
	BonusText      string // 当前生效 bonus 文案(如 "Top up $50 get $30 free"),由调用方按语言生成
	UnsubscribeURL string
}

const emailDefaultLang = "en"

// emailTemplates: lang → step → 模板。
// 文案为用户可见、非控制台文案,独立于 go-i18n 体系(spec §9)。
// 覆盖投放市场语言 en/zh/pt/es/ja;de 等不支持语言在 RenderEmail 中回退 en。
var emailTemplates = map[string]map[int]EmailTemplate{
	"en": {
		1: {
			Subject: "Welcome to {{.SystemName}} — make your first API call in 30s",
			BodyHTML: `<p>Welcome to {{.SystemName}}!</p>
<p>You're 30 seconds away from your first API call.{{if .BonusText}} <strong>{{.BonusText}}</strong>{{end}}</p>
<p><a href="{{.SignUpLink}}">Create your API Key</a> &middot; <a href="{{.QuickstartLink}}">View Quickstart</a></p>
<hr><p style="font-size:12px;color:#888">Don't want these emails? <a href="{{.UnsubscribeURL}}">Unsubscribe</a></p>`,
		},
		2: {
			Subject: "{{.SystemName}}: you haven't made your first call yet",
			BodyHTML: `<p>Hi, we noticed you haven't made your first API call yet.</p>
<p>It only takes a minute.{{if .BonusText}} <strong>{{.BonusText}}</strong>{{end}}</p>
<p><a href="{{.QuickstartLink}}">Try it now</a></p>
<hr><p style="font-size:12px;color:#888"><a href="{{.UnsubscribeURL}}">Unsubscribe</a></p>`,
		},
		3: {
			Subject: "{{.SystemName}}: a bigger bonus is waiting for you",
			BodyHTML: `<p>Still exploring? Here's an extra incentive.</p>
<p><strong>{{.BonusText}}</strong></p>
<p><a href="{{.TopupLink}}">Top up &amp; claim bonus</a></p>
<hr><p style="font-size:12px;color:#888"><a href="{{.UnsubscribeURL}}">Unsubscribe</a></p>`,
		},
		4: {
			Subject: "{{.SystemName}}: last chance — our top bonus tier",
			BodyHTML: `<p>This is our final reminder — the best bonus we offer.</p>
<p><strong>{{.BonusText}}</strong></p>
<p><a href="{{.TopupLink}}">Last chance, claim your bonus</a></p>
<hr><p style="font-size:12px;color:#888"><a href="{{.UnsubscribeURL}}">Unsubscribe</a></p>`,
		},
	},
	"zh": {
		1: {
			Subject: "欢迎使用 {{.SystemName}} —— 30 秒完成首次 API 调用",
			BodyHTML: `<p>欢迎使用 {{.SystemName}}！</p>
<p>距离你的第一次 API 调用只差 30 秒。{{if .BonusText}} <strong>{{.BonusText}}</strong>{{end}}</p>
<p><a href="{{.SignUpLink}}">立即创建 API Key</a> &middot; <a href="{{.QuickstartLink}}">查看快速上手</a></p>
<hr><p style="font-size:12px;color:#888">不想再收到这些邮件？<a href="{{.UnsubscribeURL}}">退订</a></p>`,
		},
		2: {
			Subject: "{{.SystemName}}：你还没有发起第一次调用",
			BodyHTML: `<p>你好，我们注意到你还没有发起第一次 API 调用。</p>
<p>只需一分钟即可完成。{{if .BonusText}} <strong>{{.BonusText}}</strong>{{end}}</p>
<p><a href="{{.QuickstartLink}}">立即试调</a></p>
<hr><p style="font-size:12px;color:#888"><a href="{{.UnsubscribeURL}}">退订</a></p>`,
		},
		3: {
			Subject: "{{.SystemName}}：更高的充值赠送在等你",
			BodyHTML: `<p>还在观望吗？这里有一份额外奖励。</p>
<p><strong>{{.BonusText}}</strong></p>
<p><a href="{{.TopupLink}}">去充值领奖励</a></p>
<hr><p style="font-size:12px;color:#888"><a href="{{.UnsubscribeURL}}">退订</a></p>`,
		},
		4: {
			Subject: "{{.SystemName}}：最后机会 —— 最高档充值赠送",
			BodyHTML: `<p>这是我们的最后一次提醒 —— 也是我们提供的最高赠送档位。</p>
<p><strong>{{.BonusText}}</strong></p>
<p><a href="{{.TopupLink}}">最后机会，立即领取奖励</a></p>
<hr><p style="font-size:12px;color:#888"><a href="{{.UnsubscribeURL}}">退订</a></p>`,
		},
	},
	"pt": {
		1: {
			Subject: "Bem-vindo ao {{.SystemName}} — faça sua primeira chamada de API em 30s",
			BodyHTML: `<p>Bem-vindo ao {{.SystemName}}!</p>
<p>Você está a 30 segundos da sua primeira chamada de API.{{if .BonusText}} <strong>{{.BonusText}}</strong>{{end}}</p>
<p><a href="{{.SignUpLink}}">Crie sua API Key</a> &middot; <a href="{{.QuickstartLink}}">Ver guia rápido</a></p>
<hr><p style="font-size:12px;color:#888">Não quer mais estes e-mails? <a href="{{.UnsubscribeURL}}">Cancelar inscrição</a></p>`,
		},
		2: {
			Subject: "{{.SystemName}}: você ainda não fez sua primeira chamada",
			BodyHTML: `<p>Olá, percebemos que você ainda não fez sua primeira chamada de API.</p>
<p>Leva apenas um minuto.{{if .BonusText}} <strong>{{.BonusText}}</strong>{{end}}</p>
<p><a href="{{.QuickstartLink}}">Experimente agora</a></p>
<hr><p style="font-size:12px;color:#888"><a href="{{.UnsubscribeURL}}">Cancelar inscrição</a></p>`,
		},
		3: {
			Subject: "{{.SystemName}}: um bônus maior está esperando por você",
			BodyHTML: `<p>Ainda explorando? Aqui está um incentivo extra.</p>
<p><strong>{{.BonusText}}</strong></p>
<p><a href="{{.TopupLink}}">Recarregue e resgate o bônus</a></p>
<hr><p style="font-size:12px;color:#888"><a href="{{.UnsubscribeURL}}">Cancelar inscrição</a></p>`,
		},
		4: {
			Subject: "{{.SystemName}}: última chance — nosso maior bônus",
			BodyHTML: `<p>Este é o nosso lembrete final — o melhor bônus que oferecemos.</p>
<p><strong>{{.BonusText}}</strong></p>
<p><a href="{{.TopupLink}}">Última chance, resgate seu bônus</a></p>
<hr><p style="font-size:12px;color:#888"><a href="{{.UnsubscribeURL}}">Cancelar inscrição</a></p>`,
		},
	},
	"es": {
		1: {
			Subject: "Bienvenido a {{.SystemName}} — haz tu primera llamada de API en 30s",
			BodyHTML: `<p>¡Bienvenido a {{.SystemName}}!</p>
<p>Estás a 30 segundos de tu primera llamada de API.{{if .BonusText}} <strong>{{.BonusText}}</strong>{{end}}</p>
<p><a href="{{.SignUpLink}}">Crea tu API Key</a> &middot; <a href="{{.QuickstartLink}}">Ver guía rápida</a></p>
<hr><p style="font-size:12px;color:#888">¿No quieres estos correos? <a href="{{.UnsubscribeURL}}">Darse de baja</a></p>`,
		},
		2: {
			Subject: "{{.SystemName}}: aún no has hecho tu primera llamada",
			BodyHTML: `<p>Hola, hemos notado que aún no has hecho tu primera llamada de API.</p>
<p>Solo toma un minuto.{{if .BonusText}} <strong>{{.BonusText}}</strong>{{end}}</p>
<p><a href="{{.QuickstartLink}}">Pruébalo ahora</a></p>
<hr><p style="font-size:12px;color:#888"><a href="{{.UnsubscribeURL}}">Darse de baja</a></p>`,
		},
		3: {
			Subject: "{{.SystemName}}: un bono mayor te está esperando",
			BodyHTML: `<p>¿Aún explorando? Aquí tienes un incentivo extra.</p>
<p><strong>{{.BonusText}}</strong></p>
<p><a href="{{.TopupLink}}">Recarga y reclama el bono</a></p>
<hr><p style="font-size:12px;color:#888"><a href="{{.UnsubscribeURL}}">Darse de baja</a></p>`,
		},
		4: {
			Subject: "{{.SystemName}}: última oportunidad — nuestro bono más alto",
			BodyHTML: `<p>Este es nuestro recordatorio final — el mejor bono que ofrecemos.</p>
<p><strong>{{.BonusText}}</strong></p>
<p><a href="{{.TopupLink}}">Última oportunidad, reclama tu bono</a></p>
<hr><p style="font-size:12px;color:#888"><a href="{{.UnsubscribeURL}}">Darse de baja</a></p>`,
		},
	},
	"ja": {
		1: {
			Subject: "{{.SystemName}} へようこそ — 30秒で最初のAPI呼び出しを",
			BodyHTML: `<p>{{.SystemName}} へようこそ！</p>
<p>最初のAPI呼び出しまであと30秒です。{{if .BonusText}} <strong>{{.BonusText}}</strong>{{end}}</p>
<p><a href="{{.SignUpLink}}">APIキーを作成</a> &middot; <a href="{{.QuickstartLink}}">クイックスタートを見る</a></p>
<hr><p style="font-size:12px;color:#888">このメールの配信を停止しますか？<a href="{{.UnsubscribeURL}}">配信停止</a></p>`,
		},
		2: {
			Subject: "{{.SystemName}}：まだ最初の呼び出しが行われていません",
			BodyHTML: `<p>こんにちは。まだ最初のAPI呼び出しが行われていないようです。</p>
<p>ほんの1分で完了します。{{if .BonusText}} <strong>{{.BonusText}}</strong>{{end}}</p>
<p><a href="{{.QuickstartLink}}">今すぐ試す</a></p>
<hr><p style="font-size:12px;color:#888"><a href="{{.UnsubscribeURL}}">配信停止</a></p>`,
		},
		3: {
			Subject: "{{.SystemName}}：より大きなボーナスをご用意しています",
			BodyHTML: `<p>まだご検討中ですか？特典をご用意しました。</p>
<p><strong>{{.BonusText}}</strong></p>
<p><a href="{{.TopupLink}}">チャージしてボーナスを受け取る</a></p>
<hr><p style="font-size:12px;color:#888"><a href="{{.UnsubscribeURL}}">配信停止</a></p>`,
		},
		4: {
			Subject: "{{.SystemName}}：最後のチャンス — 最高ランクのボーナス",
			BodyHTML: `<p>これが最後のお知らせです — 当社で最も手厚いボーナスです。</p>
<p><strong>{{.BonusText}}</strong></p>
<p><a href="{{.TopupLink}}">最後のチャンス、ボーナスを受け取る</a></p>
<hr><p style="font-size:12px;color:#888"><a href="{{.UnsubscribeURL}}">配信停止</a></p>`,
		},
	},
}

func getEmailTemplate(lang string, step int) (EmailTemplate, bool) {
	if m, ok := emailTemplates[lang]; ok {
		if tpl, ok := m[step]; ok {
			return tpl, true
		}
	}
	return EmailTemplate{}, false
}

// RenderEmail 渲染指定语言+step 的邮件。lang 不支持时回退 en。
func RenderEmail(lang string, step int, data EmailRenderData) (string, string, error) {
	tpl, ok := getEmailTemplate(lang, step)
	if !ok {
		tpl, ok = getEmailTemplate(emailDefaultLang, step)
		if !ok {
			return "", "", nil
		}
	}
	subject, err := renderTemplateStr(tpl.Subject, data)
	if err != nil {
		return "", "", err
	}
	body, err := renderTemplateStr(tpl.BodyHTML, data)
	if err != nil {
		return "", "", err
	}
	return subject, body, nil
}

func renderTemplateStr(tplStr string, data EmailRenderData) (string, error) {
	t, err := template.New("email").Parse(tplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
