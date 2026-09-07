package quizparser

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/egot3/fathom/internal/quiz"
	"go.yaml.in/yaml/v4"
)

func ParseFrontmatter(scanner *bufio.Scanner) (quiz.Frontmatter, error) {
	var fm quiz.Frontmatter

	if strings.TrimSpace(scanner.Text()) != "---" {
		for {
			if !scanner.Scan() {
				return fm, fmt.Errorf("missing frontmatter delimeter(---)")
			}
			if strings.TrimSpace(scanner.Text()) == "---" {
				break
			}
		}
	}

	rawFrontmatter := bytes.Buffer{}
	end := -1
	for i := 0; scanner.Scan(); i++ {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			end = i
			break
		}
		rawFrontmatter.WriteString(line)
		rawFrontmatter.WriteString("\n")
	}
	if end == -1 {
		return fm, fmt.Errorf("unclosed frontmatter")
	}

	if err := yaml.NewDecoder(&rawFrontmatter).Decode(&fm); err != nil {
		return fm, err
	}
	if fm.Kind == "" {
		return fm, fmt.Errorf("mising entry in frontmatter: kind")
	}
	if fm.Score <= 0 {
		return fm, fmt.Errorf("missing score/score set to zero(or lower) in frontmatter")
	}

	return fm, nil
}
