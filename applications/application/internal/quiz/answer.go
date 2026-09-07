package quiz

type AnswerInput struct {
	Input string `json:"input"`
}

type AnswerRadio struct {
	ChoiceIdx int `json:"chosen"`
}

type AnswerCheck struct {
	ChoiceIdxs []int `json:"chosen"`
}

type AnswerAccordance struct {
	Accordance []int `json:"accorded"`
}

type AnswerOrder struct {
	ItemIdxs []int `json:"item_indexes"`
}

type QuizAnswers struct {
	Radio      *AnswerRadio      `json:"radio,omitempty"`
	Check      *AnswerCheck      `json:"check,omitempty"`
	Accordance *AnswerAccordance `json:"accordance,omitempty"`
	Order      *AnswerOrder      `json:"order,omitempty"`
	Input      *AnswerInput      `json:"input,omitempty"`
}
