# inputclf

Go-библиотека: предсказывает, принимает ли функция пользовательский ввод.
Это инференс модели из `train.py` (TF-IDF + LogisticRegression) без Python.

```go
res, err := inputclf.Predict(code, "php")
// res.AcceptsInput — true, если parsed=1
// res.Probability  — P(parsed=1)
// res.Function     — имя найденной функции
```

`lang` — расширение или имя файла (`"php"`, `".php"`, `"handler.php"`).
Если нужна функция по номерам строк в файле, как в `train.py --predict --start --end`:

```go
res, err := inputclf.PredictLines(fileBytes, "handler.php", 66, 80)
```

## Подготовка: экспорт модели

Go не читает pickle, поэтому веса один раз выгружаются в `model.json.gz`,
который вшивается в бинарник. Из папки с `train.py`, `features.py`, `normalize.py`:

```sh
python /path/to/inputclf/tools/export_model.py --out /path/to/inputclf \
    --csv ../../dataset/accepts.csv ../../dataset/src \
    --csv ../../dataset/not-accept.csv ../../dataset/src
```

Без `model.json.gz` пакет не соберётся. После каждого переобучения экспорт
нужно повторять.

## Проверка совпадения с Python

```sh
go test -v .
```

- `TestClassifierParity` — вероятности на сэмплах из `dataset.jsonl` совпадают со sklearn (до 1e-9).
- `TestNormalizeParity` — нормализация исходников из CSV (флаг `--csv`) совпадает с `normalize.py`.

## Важно

- Нужен cgo (компилятор C): tree-sitter написан на C.
- Версии грамматик в `go.mod` должны совпадать с теми, на которых обучалась
  модель (`pip list | grep tree-sitter`), иначе дерево разбора и признаки могут
  отличаться. Расхождения ловит `TestNormalizeParity`.
- `normalize.go` — порт `normalize.py`; правки в одном нужно повторять в другом.
- Имя модуля в `go.mod` (`inputclf`) замените на свой путь, например
  `github.com/you/inputclf`.
