// Package inputclf предсказывает, принимает ли функция пользовательский ввод.
//
// Это Go-инференс sklearn-модели из train.py (TF-IDF + LogisticRegression).
// Веса лежат в model.json.gz, который создаётся скриптом tools/export_model.py
// из model.pkl и вшивается в бинарник.
package inputclf

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
)

//go:embed model.json.gz
var modelGz []byte

// Result — результат предсказания.
type Result struct {
	AcceptsInput bool    // true → parsed=1 (функция принимает пользовательский ввод)
	Probability  float64 // P(parsed=1), от 0 до 1
	Function     string  // имя найденной функции
}

// Predict принимает исходный код функции и язык и возвращает предсказание модели.
//
// lang — расширение или имя файла: "php", ".php", "handler.php".
// Если lang == "", язык определяется автоматически через Detect.
// Если в code несколько функций, берётся та, что занимает больше всего строк;
// чтобы выбрать функцию по номерам строк в файле, используйте PredictLines.
// Безопасна для вызова из нескольких горутин.
func Predict(code []byte, lang string) (Result, error) {
	if lang == "" {
		lang = Detect(code)
		if lang == "" {
			return Result{}, fmt.Errorf("inputclf: cannot detect language")
		}
	}
	// Грамматика PHP разбирает код только после открывающего тега.
	if extension(lang) == ".php" && !bytes.Contains(code, []byte("<?")) {
		code = append([]byte("<?php "), code...)
	}
	return PredictLines(code, lang, 1, bytes.Count(code, []byte("\n"))+1)
}

// PredictLines — то же, что Predict, но для функции, расположенной в строках
// start..end файла src (нумерация с 1), как `train.py --predict --start --end`.
func PredictLines(src []byte, lang string, start, end int) (Result, error) {
	m, err := loadModel()
	if err != nil {
		return Result{}, err
	}
	s, err := normalize(src, extension(lang), start, end)
	if err != nil {
		return Result{}, err
	}
	p := m.proba(s)
	return Result{AcceptsInput: p > 0.5, Probability: p, Function: s.Name}, nil
}

func extension(lang string) string {
	if i := strings.LastIndexByte(lang, '.'); i >= 0 {
		lang = lang[i+1:]
	}
	return "." + strings.ToLower(lang)
}

// ---------- модель ----------

type field struct {
	Name        string                `json:"name"` // body | names | params | meta
	NgramMin    int                   `json:"ngram_min"`
	NgramMax    int                   `json:"ngram_max"`
	SublinearTF bool                  `json:"sublinear_tf"`
	Lowercase   bool                  `json:"lowercase"`
	Terms       map[string][2]float64 `json:"terms"` // терм → [idf, вес LogisticRegression]
}

type model struct {
	Intercept float64 `json:"intercept"`
	Fields    []field `json:"fields"`
}

var (
	modelOnce sync.Once
	theModel  *model
	modelErr  error
)

func loadModel() (*model, error) {
	modelOnce.Do(func() {
		zr, err := gzip.NewReader(bytes.NewReader(modelGz))
		if err != nil {
			modelErr = fmt.Errorf("inputclf: model.json.gz: %w", err)
			return
		}
		m := &model{}
		if err := json.NewDecoder(zr).Decode(m); err != nil {
			modelErr = fmt.Errorf("inputclf: model.json.gz: %w", err)
			return
		}
		for _, f := range m.Fields {
			if _, ok := fieldText[f.Name]; !ok {
				modelErr = fmt.Errorf("inputclf: model has unknown field %q", f.Name)
				return
			}
		}
		theModel = m
	})
	return theModel, modelErr
}

// Тексты полей — повторяют _body/_name_tokens/_params/_meta из features.py.
var fieldText = map[string]func(*sample) string{
	"body":  func(s *sample) string { return s.Body },
	"names": func(s *sample) string { return strings.Join(s.NameTokens, " ") },
	"params": func(s *sample) string {
		parts := make([]string, len(s.Params))
		for i, p := range s.Params {
			parts[i] = p.Name + " " + p.Type
		}
		return strings.Join(parts, " ")
	},
	"meta": func(s *sample) string {
		return fmt.Sprintf("np%d bl%d", len(s.Params), min(len(fields(s.Body))/10, 20))
	},
}

// proba = sigmoid(w · tfidf(sample) + b), как pipe.predict_proba(...)[0][1].
func (m *model) proba(s *sample) float64 {
	z := m.Intercept
	for i := range m.Fields {
		f := &m.Fields[i]
		z += f.score(fieldText[f.Name](s))
	}
	return 1 / (1 + math.Exp(-z))
}

// score — вклад одного TfidfVectorizer в решающую функцию:
// L2-нормированный вектор tf·idf, умноженный на веса классификатора.
func (f *field) score(txt string) float64 {
	if f.Lowercase {
		txt = strings.ToLower(txt)
	}
	toks := fields(txt) // token_pattern=r"[^\s]+"
	counts := map[string]int{}
	for n := f.NgramMin; n <= f.NgramMax; n++ {
		for i := 0; i+n <= len(toks); i++ {
			term := toks[i]
			if n > 1 {
				term = strings.Join(toks[i:i+n], " ")
			}
			if _, ok := f.Terms[term]; ok {
				counts[term]++
			}
		}
	}
	var dot, sumSq float64
	for term, c := range counts {
		tf := float64(c)
		if f.SublinearTF {
			tf = 1 + math.Log(tf)
		}
		t := f.Terms[term]
		v := tf * t[0]
		dot += v * t[1]
		sumSq += v * v
	}
	if sumSq == 0 {
		return 0
	}
	return dot / math.Sqrt(sumSq)
}
