package quiz

import "slices"

func evaluateInput(required, factual *AnswerInput, score int) float32 {
	if required.Input == factual.Input {
		return float32(score)
	}

	return 0
}

func evaluateRadio(required, factual *AnswerRadio, score int) float32 {
	if required.ChoiceIdx == factual.ChoiceIdx {
		return float32(score)
	}

	return 0
}

func evaluateCheck(required, factual *AnswerCheck, score int) float32 {
	total := float32(0)
	if len(required.ChoiceIdxs) < len(factual.ChoiceIdxs) {
		return 0
	}

	var iterDist = len(factual.ChoiceIdxs)

	for i := 0; i < iterDist; i++ {
		if slices.Contains(factual.ChoiceIdxs, required.ChoiceIdxs[i]) {
			total++
		}
	}

	return float32(score) * (total / float32(iterDist))
}

func evaluateAllOrNoneCheck(required, factual *AnswerCheck, score int) float32 {
	if len(required.ChoiceIdxs) < len(factual.ChoiceIdxs) {
		return 0
	}

	var iterDist = len(factual.ChoiceIdxs)

	for i := 0; i < iterDist; i++ {
		if !slices.Contains(factual.ChoiceIdxs, required.ChoiceIdxs[i]) {
			return 0
		}
	}

	return float32(score)
}

func evaluateOrder(required, factual *AnswerOrder, score int) float32 {
	total := float32(0)
	if len(required.ItemIdxs) < len(factual.ItemIdxs) {
		return 0
	}

	var iterDist = len(factual.ItemIdxs)

	for i := 0; i < iterDist; i++ {
		if required.ItemIdxs[i] == factual.ItemIdxs[i] {
			total++
		}
	}

	return float32(score) * (total / float32(iterDist))
}

func evaluateAllOrNoneOrder(required, factual *AnswerOrder, score int) float32 {
	if len(required.ItemIdxs) < len(factual.ItemIdxs) {
		return 0
	}

	var iterDist = len(factual.ItemIdxs)

	for i := 0; i < iterDist; i++ {
		if required.ItemIdxs[i] != factual.ItemIdxs[i] {
			return 0
		}
	}

	return float32(score)
}

func evaluateAccordance(required, factual *AnswerAccordance, score int) float32 {
	total := float32(0)
	if len(required.Accordance) < len(factual.Accordance) {
		return 0
	}

	var iterDist = len(factual.Accordance)

	for i := 0; i < iterDist; i++ {
		if required.Accordance[i] == factual.Accordance[i] {
			total++
		}
	}

	return float32(score) * (total / float32(iterDist))
}

func evaluateAllOrNoneAccordance(required, factual *AnswerAccordance, score int) float32 {
	if slices.Equal(required.Accordance, factual.Accordance) {
		return float32(score)
	}

	return 0
}
