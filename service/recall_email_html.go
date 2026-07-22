package service

import (
	"bytes"
	"fmt"
	htmltemplate "html/template"
	"net/url"
	"strings"
	texttemplate "text/template"
	"text/template/parse"

	"golang.org/x/net/html"
)

const recallEmailHTMLMaxBytes = 100 * 1024

var recallEmailHTMLTemplateFields = map[string]struct{}{
	"RecipientName":       {},
	"PromotionCodeMasked": {},
	"ProductSummary":      {},
	"ExpiresAt":           {},
	"ClaimURL":            {},
	"UnsubscribeURL":      {},
}

type recallEmailHTMLDocument struct {
	source string
	root   *html.Node
	slots  []recallEmailHTMLSlot
}

type recallEmailHTMLSlot struct {
	node      *html.Node
	attrIndex int
	value     string
}

func parseRecallEmailHTML(source string) (*recallEmailHTMLDocument, error) {
	if len([]byte(source)) > recallEmailHTMLMaxBytes {
		return nil, fmt.Errorf("recall email html must contain at most %d bytes", recallEmailHTMLMaxBytes)
	}
	if err := validateRecallEmailTemplateActions(source); err != nil {
		return nil, err
	}
	root, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return nil, fmt.Errorf("parse recall email html: %w", err)
	}
	document := &recallEmailHTMLDocument{source: source, root: root}
	foundClaimURL := false
	foundUnsubscribeURL := false
	if err := walkRecallEmailHTML(root, false, false, document, &foundClaimURL, &foundUnsubscribeURL); err != nil {
		return nil, err
	}
	if !foundClaimURL {
		return nil, fmt.Errorf("ClaimURL action must appear in an anchor href")
	}
	if !foundUnsubscribeURL {
		return nil, fmt.Errorf("UnsubscribeURL action must appear in an anchor href")
	}
	return document, nil
}

func validateRecallEmailTemplateBodyContract(template RecallEmailTemplate) (RecallEmailTemplate, error) {
	template.Subject = strings.TrimSpace(template.Subject)
	template.BodyText = strings.TrimSpace(template.BodyText)
	template.BodyHTML = strings.TrimSpace(template.BodyHTML)
	if template.Subject == "" {
		return RecallEmailTemplate{}, fmt.Errorf("recall email template requires subject")
	}
	if (template.BodyText == "") == (template.BodyHTML == "") {
		return RecallEmailTemplate{}, fmt.Errorf("recall email template requires exactly one body")
	}
	if strings.ContainsAny(template.Subject, "\r\n") {
		return RecallEmailTemplate{}, fmt.Errorf("recall email template subject must be single line")
	}
	if template.BodyHTML != "" {
		if _, err := parseRecallEmailHTML(template.BodyHTML); err != nil {
			return RecallEmailTemplate{}, fmt.Errorf("recall email template body html: %w", err)
		}
	}
	return template, nil
}

func (document *recallEmailHTMLDocument) TranslationSegments() []string {
	segments := make([]string, len(document.slots))
	for index, slot := range document.slots {
		segments[index] = slot.value
	}
	return segments
}

func (document *recallEmailHTMLDocument) Rebuild(translations []string) (string, error) {
	if len(translations) != len(document.slots) {
		return "", fmt.Errorf("recall email html translation count %d does not match %d slots", len(translations), len(document.slots))
	}
	cloneMap := make(map[*html.Node]*html.Node)
	root := cloneRecallEmailHTMLNode(document.root, cloneMap)
	for index, slot := range document.slots {
		node := cloneMap[slot.node]
		if node == nil {
			return "", fmt.Errorf("recall email html slot node missing")
		}
		if slot.attrIndex >= 0 {
			node.Attr[slot.attrIndex].Val = translations[index]
			continue
		}
		node.Data = translations[index]
	}
	var rendered bytes.Buffer
	if err := html.Render(&rendered, root); err != nil {
		return "", fmt.Errorf("render recall email html: %w", err)
	}
	if rendered.Len() > recallEmailHTMLMaxBytes {
		return "", fmt.Errorf("recall email html must contain at most %d bytes", recallEmailHTMLMaxBytes)
	}
	output := rendered.String()
	if _, err := parseRecallEmailHTML(output); err != nil {
		return "", err
	}
	return output, nil
}

func walkRecallEmailHTML(node *html.Node, inHead bool, inStyle bool, document *recallEmailHTMLDocument, foundClaimURL *bool, foundUnsubscribeURL *bool) error {
	nextInHead := inHead
	nextInStyle := inStyle
	if node.Type == html.ElementNode {
		element := strings.ToLower(node.Data)
		nextInHead = inHead || element == "head"
		nextInStyle = inStyle || element == "style"
		if err := validateRecallEmailHTMLElement(node); err != nil {
			return err
		}
		for index, attr := range node.Attr {
			if err := validateRecallEmailHTMLAttribute(element, attr); err != nil {
				return err
			}
			value := strings.TrimSpace(attr.Val)
			key := strings.ToLower(attr.Key)
			actions, err := recallEmailHTMLURLActionsInTemplate(attr.Val)
			if err != nil {
				return err
			}
			if actions.claim || actions.unsubscribe {
				if element != "a" || key != "href" || (value != "{{.ClaimURL}}" && value != "{{.UnsubscribeURL}}") {
					return actions.err()
				}
			}
			if element == "a" && key == "href" {
				switch value {
				case "{{.ClaimURL}}":
					*foundClaimURL = true
				case "{{.UnsubscribeURL}}":
					*foundUnsubscribeURL = true
				}
			}
			if isRecallEmailHTMLTranslatableAttribute(key) && strings.TrimSpace(attr.Val) != "" {
				document.slots = append(document.slots, recallEmailHTMLSlot{node: node, attrIndex: index, value: attr.Val})
			}
		}
	}
	if node.Type == html.TextNode || node.Type == html.CommentNode {
		actions, err := recallEmailHTMLURLActionsInTemplate(node.Data)
		if err != nil {
			return err
		}
		if actions.claim || actions.unsubscribe {
			return actions.err()
		}
	}
	if node.Type == html.TextNode && !inHead && !inStyle && strings.TrimSpace(node.Data) != "" {
		document.slots = append(document.slots, recallEmailHTMLSlot{node: node, attrIndex: -1, value: node.Data})
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if err := walkRecallEmailHTML(child, nextInHead, nextInStyle, document, foundClaimURL, foundUnsubscribeURL); err != nil {
			return err
		}
	}
	return nil
}

type recallEmailHTMLURLActions struct {
	claim       bool
	unsubscribe bool
}

func (actions recallEmailHTMLURLActions) err() error {
	if actions.claim {
		return fmt.Errorf("ClaimURL action must appear in an anchor href")
	}
	return fmt.Errorf("UnsubscribeURL action must appear in an anchor href")
}

func recallEmailHTMLURLActionsInTemplate(raw string) (recallEmailHTMLURLActions, error) {
	if !strings.Contains(raw, "{{") {
		return recallEmailHTMLURLActions{}, nil
	}
	template, err := texttemplate.New("recall-email-html-fragment").Parse(raw)
	if err != nil {
		return recallEmailHTMLURLActions{}, fmt.Errorf("parse recall email html template fragment: %w", err)
	}
	actions := recallEmailHTMLURLActions{}
	collectRecallEmailHTMLURLActions(template.Tree.Root, &actions)
	return actions, nil
}

func collectRecallEmailHTMLURLActions(node parse.Node, actions *recallEmailHTMLURLActions) {
	switch typed := node.(type) {
	case *parse.ListNode:
		for _, child := range typed.Nodes {
			collectRecallEmailHTMLURLActions(child, actions)
		}
	case *parse.ActionNode:
		if typed.Pipe != nil {
			collectRecallEmailHTMLURLActions(typed.Pipe, actions)
		}
	case *parse.PipeNode:
		for _, command := range typed.Cmds {
			collectRecallEmailHTMLURLActions(command, actions)
		}
	case *parse.CommandNode:
		for _, arg := range typed.Args {
			collectRecallEmailHTMLURLActions(arg, actions)
		}
	case *parse.FieldNode:
		if len(typed.Ident) == 1 {
			switch typed.Ident[0] {
			case "ClaimURL":
				actions.claim = true
			case "UnsubscribeURL":
				actions.unsubscribe = true
			}
		}
	}
}

func validateRecallEmailHTMLElement(node *html.Node) error {
	element := strings.ToLower(node.Data)
	if node.Namespace != "" || element == "svg" || element == "math" {
		return fmt.Errorf("recall email html rejects %s elements", element)
	}
	switch element {
	case "script", "iframe", "object", "embed", "form", "base",
		"input", "button", "select", "textarea", "option", "optgroup", "fieldset", "datalist", "output":
		return fmt.Errorf("recall email html rejects %s elements", element)
	case "meta":
		for _, attr := range node.Attr {
			if strings.EqualFold(attr.Key, "http-equiv") && strings.EqualFold(strings.TrimSpace(attr.Val), "refresh") {
				return fmt.Errorf("recall email html rejects meta refresh")
			}
		}
	case "style":
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.TextNode {
				if err := validateRecallEmailCSS(child.Data); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func validateRecallEmailHTMLAttribute(element string, attr html.Attribute) error {
	key := strings.ToLower(attr.Key)
	if strings.HasPrefix(key, "on") {
		return fmt.Errorf("recall email html rejects event handler attribute %q", attr.Key)
	}
	if key == "srcdoc" {
		return fmt.Errorf("recall email html rejects srcdoc")
	}
	if key == "style" {
		if err := validateRecallEmailCSS(attr.Val); err != nil {
			return err
		}
	}
	switch key {
	case "href", "src", "background", "poster":
		if err := validateRecallEmailURL(attr.Val, element == "a" && key == "href"); err != nil {
			return err
		}
	}
	return nil
}

func validateRecallEmailURL(raw string, allowDynamic bool) error {
	value := strings.TrimSpace(raw)
	if allowDynamic && (value == "{{.ClaimURL}}" || value == "{{.UnsubscribeURL}}") {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" {
		return fmt.Errorf("recall email html urls must be absolute http or https")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("recall email html urls must be absolute http or https")
	}
	return nil
}

func validateRecallEmailCSS(raw string) error {
	normalized := normalizeRecallEmailCSS(raw)
	unsafe := []string{"expression(", "javascript:", "vbscript:", "data:", "@import", "behavior:", "-moz-binding"}
	for _, token := range unsafe {
		if strings.Contains(normalized, token) {
			return fmt.Errorf("recall email html contains unsafe css")
		}
	}
	return nil
}

func validateRecallEmailTemplateActions(source string) error {
	template, err := htmltemplate.New("recall-email-html").Option("missingkey=error").Parse(source)
	if err != nil {
		return fmt.Errorf("parse recall email html template: %w", err)
	}
	if template.Tree == nil || template.Tree.Root == nil {
		return nil
	}
	return validateRecallEmailTemplateNode(template.Tree.Root)
}

func validateRecallEmailTemplateNode(node parse.Node) error {
	switch typed := node.(type) {
	case *parse.ListNode:
		for _, child := range typed.Nodes {
			if err := validateRecallEmailTemplateNode(child); err != nil {
				return err
			}
		}
	case *parse.TextNode:
		return nil
	case *parse.ActionNode:
		if typed.Pipe == nil {
			return fmt.Errorf("unsupported template command")
		}
		if len(typed.Pipe.Decl) != 0 {
			return fmt.Errorf("unsupported template variable")
		}
		if len(typed.Pipe.Cmds) != 1 {
			return fmt.Errorf("unsupported template command")
		}
		command := typed.Pipe.Cmds[0]
		if len(command.Args) != 1 {
			return fmt.Errorf("unsupported template command")
		}
		field, ok := command.Args[0].(*parse.FieldNode)
		if !ok {
			return fmt.Errorf("unsupported template command")
		}
		if len(field.Ident) != 1 {
			return fmt.Errorf("unsupported template field")
		}
		if _, allowed := recallEmailHTMLTemplateFields[field.Ident[0]]; !allowed {
			return fmt.Errorf("unsupported template field %q", field.Ident[0])
		}
	case *parse.IfNode, *parse.RangeNode, *parse.WithNode, *parse.TemplateNode:
		return fmt.Errorf("unsupported template control")
	default:
		return fmt.Errorf("unsupported template command")
	}
	return nil
}

func normalizeRecallEmailCSS(raw string) string {
	var builder strings.Builder
	inComment := false
	for index := 0; index < len(raw); index++ {
		if inComment {
			if raw[index] == '*' && index+1 < len(raw) && raw[index+1] == '/' {
				inComment = false
				index++
			}
			continue
		}
		if raw[index] == '/' && index+1 < len(raw) && raw[index+1] == '*' {
			inComment = true
			index++
			continue
		}
		switch raw[index] {
		case ' ', '\t', '\n', '\r', '\f':
			continue
		default:
			builder.WriteByte(raw[index])
		}
	}
	return strings.ToLower(builder.String())
}

func isRecallEmailHTMLTranslatableAttribute(key string) bool {
	switch key {
	case "alt", "title", "aria-label":
		return true
	default:
		return false
	}
}

func cloneRecallEmailHTMLNode(node *html.Node, cloneMap map[*html.Node]*html.Node) *html.Node {
	clone := &html.Node{
		Type:      node.Type,
		DataAtom:  node.DataAtom,
		Data:      node.Data,
		Namespace: node.Namespace,
	}
	if len(node.Attr) > 0 {
		clone.Attr = append([]html.Attribute(nil), node.Attr...)
	}
	cloneMap[node] = clone
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		clone.AppendChild(cloneRecallEmailHTMLNode(child, cloneMap))
	}
	return clone
}
