package quizparser

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/egot3/fathom/internal/quiz"
)

func ParseQuiz(reader io.Reader) (*quiz.Quiz, error) {

	scanner := bufio.NewScanner(reader)

	var q quiz.Quiz
	fm, err := ParseFrontmatter(scanner)
	if err != nil {
		return nil, err
	}

	q.Meta = fm

	for scanner.Scan() {
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimmedLine, "# ") {
			q.Title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			break
		}
	}

	q.Body = strings.TrimSpace(ParseBody(scanner))

	// each quiz has 1 typeof question
	switch q.Meta.Kind {
	case quiz.Input:
		if err := InputParser(scanner, &q); err != nil {
			return nil, err
		}
	case quiz.Check:
		if err := CheckParser(scanner, &q); err != nil {
			return nil, err
		}
	case quiz.Radio:
		if err := RadioParser(scanner, &q); err != nil {
			return nil, err
		}
	case quiz.Order:
		if err := OrderParser(scanner, &q); err != nil {
			return nil, err
		}
	case quiz.Accordance:
		if err := AccordanceParser(scanner, &q); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("Unsupported kind")
	}

	return &q, nil
}

func ParseBody(scanner *bufio.Scanner) string {
	var sb strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)

		if InputRegex.MatchString(trimmedLine) || strings.HasPrefix(trimmedLine, "- ") { // using "- " because all of kinds(with sole exception of Input, of course) have this
			break
		}

		sb.WriteString(trimmedLine)
		sb.WriteRune(' ')
	}

	return sb.String()
}
