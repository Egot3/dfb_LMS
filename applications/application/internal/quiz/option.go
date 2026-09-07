package quiz

type OptionsRadioAndCheck struct {
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Id    int    `json:"id"`
	Label string `json:"label"`
}

type OptionsAccordance struct {
	Static  []string `json:"static"`
	Dynamic []string `json:"dynamic"`
}

type OptionsOrder struct {
	Items []string `json:"items"`
}

type QuizOptions struct {
	Radio      *OptionsRadioAndCheck `json:"radio,omitempty"`
	Check      *OptionsRadioAndCheck `json:"check,omitempty"`
	Accordance *OptionsAccordance    `json:"accordance,omitempty"`
	Order      *OptionsOrder         `json:"orders,omitempty"`
}
