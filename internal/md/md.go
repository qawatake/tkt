// https://github.com/ankitpokhrel/jira-cli/blob/adab79ff71c7191d467818e50a84874102f4c78f/pkg/md/md.go の一部を改修しました。
package md

import (
	"github.com/ankitpokhrel/jira-cli/pkg/md/jirawiki"
	bf "github.com/russross/blackfriday/v2"
)

// ToJiraMD translates CommonMark to Jira flavored markdown.
func ToJiraMD(md string) string {
	if md == "" {
		return md
	}

	renderer := &Renderer{Flags: IgnoreMacroEscaping}
	r := bf.New(bf.WithRenderer(renderer), bf.WithExtensions(bf.CommonExtensions))

	return string(renderer.Render(r.Parse([]byte(md))))
}

// FromJiraMD translates Jira flavored markdown to CommonMark.
func FromJiraMD(jfm string) string {
	return jirawiki.Parse(jfm)
}
