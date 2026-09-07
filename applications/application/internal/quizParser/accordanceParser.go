package quizparser

import (
	"bufio"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/egot3/fathom/internal/quiz"
)

func AccordanceParser(reader *bufio.Scanner, quizP *quiz.Quiz) error {
	var keys []string
	var vals []string
	// Q: why not map[string]string?
	// A: slices are ordered, can reuse logic from order

	for {
		line := reader.Text()
		trimmedLine := strings.TrimSpace(line)

		if after, ok := strings.CutPrefix(trimmedLine, "- "); ok {
			corelation := strings.Split(strings.TrimSpace(after), "|")
			if len(corelation) != 2 {
				return fmt.Errorf("can't have not a 'key | value' corelations")
			}
			key := corelation[0]
			val := corelation[1]
			if slices.Contains(keys, key) {
				return fmt.Errorf("keys in accordance can't repeat")
			}
			if slices.Contains(vals, val) {
				return fmt.Errorf("vals in accordabnce must be unique")
			}

			keys = append(keys, key)
			vals = append(vals, val)
		}

		if !reader.Scan() {
			break
		}
	}

	if len(keys) < 2 {
		return fmt.Errorf("can't have less than 2 options for accordance")
	}

	if quizP.Meta.Randomized {
		rand.Shuffle(len(keys), func(i, j int) {
			vals[i], vals[j] = vals[j], vals[i]
			keys[i], keys[j] = keys[j], keys[i] //both are shuffled as they are order dependant
		})
	}

	shuffled := make([]string, len(vals))
	copy(shuffled, vals)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	answers := make([]int, len(shuffled))
	for i, ord := range vals {
		answers[i] = slices.Index(shuffled, ord)
	}

	quizP.Options.Accordance = &quiz.OptionsAccordance{Static: keys, Dynamic: shuffled}
	quizP.Answer.Accordance = &quiz.AnswerAccordance{Accordance: answers}

	return nil
}
