package quizparser

import (
	"bufio"
	"fmt"
	"math/rand/v2"
	"strings"

	"github.com/egot3/fathom/internal/quiz"
)

func CheckParser(reader *bufio.Scanner, quizP *quiz.Quiz) error {
	quizP.Options.Check = &quiz.OptionsRadioAndCheck{Choices: make([]quiz.Choice, 0)}
	quizP.Answer.Check = &quiz.AnswerCheck{ChoiceIdxs: make([]int, 0)}
	for id := 0; ; {
		line := reader.Text()
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "- [x]") {
			opt := strings.TrimSpace(strings.TrimPrefix(trimmedLine, "- [x] "))
			quizP.Answer.Check.ChoiceIdxs = append(quizP.Answer.Check.ChoiceIdxs, id)

			quizP.Options.Check.Choices = append(quizP.Options.Check.Choices, quiz.Choice{Id: id, Label: opt})
			id++
		}
		if strings.HasPrefix(trimmedLine, "- [ ]") {
			opt := strings.TrimSpace(strings.TrimPrefix(trimmedLine, "- [ ] "))

			quizP.Options.Check.Choices = append(quizP.Options.Check.Choices, quiz.Choice{Id: id, Label: opt})
			id++
		}

		if !reader.Scan() {
			break
		}
	}

	if len(quizP.Options.Check.Choices) < 2 {
		return fmt.Errorf("can't have less than 2 options for check")
	}
	if len(quizP.Answer.Check.ChoiceIdxs) == 0 {
		return fmt.Errorf("can't have less than 1 answer for check")
	}

	if quizP.Meta.Randomized {
		rand.Shuffle(len(quizP.Options.Check.Choices), func(i, j int) {
			quizP.Options.Check.Choices[i], quizP.Options.Check.Choices[j] = quizP.Options.Check.Choices[j], quizP.Options.Check.Choices[i]
		})
	}

	return nil
}
