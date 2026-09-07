package quizparser

import (
	"bufio"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/egot3/fathom/internal/quiz"
)

func OrderParser(reader *bufio.Scanner, quizP *quiz.Quiz) error {
	var ordered []string

	for {
		line := reader.Text()
		trimmedLine := strings.TrimSpace(line)

		if after, ok := strings.CutPrefix(trimmedLine, "- "); ok {
			item := strings.TrimSpace(after)
			if slices.Contains(ordered, item) {
				return fmt.Errorf("can't have multiple of the same value in order")
			}
			ordered = append(ordered, item)
		}
		if !reader.Scan() {
			break
		}
	}

	if len(ordered) < 2 {
		return fmt.Errorf("can't have less than 2 options for order")
	}

	shuffled := make([]string, len(ordered))
	copy(shuffled, ordered)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	answers := make([]int, len(shuffled))

	// samber's lo here cuz got it from multi-slog
	// such a waste to use it here
	for i, ord := range ordered {
		answers[i] = slices.Index(shuffled, ord)
	}

	quizP.Options.Order = &quiz.OptionsOrder{Items: shuffled}
	quizP.Answer.Order = &quiz.AnswerOrder{ItemIdxs: answers}

	return nil
}
