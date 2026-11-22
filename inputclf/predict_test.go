package inputclf

import (
	"bufio"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"
)

// Тесты на совпадение с Python. Данные создаёт tools/export_model.py;
// если файла нет, соответствующий тест пропускается.

func readJSONL(t *testing.T, path string, each func(line []byte)) {
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("%s не найден — запустите tools/export_model.py", path)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 64<<20)
	for sc.Scan() {
		each(sc.Bytes())
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
}

// Классификатор: на тех же сэмплах вероятности совпадают со sklearn.
func TestClassifierParity(t *testing.T) {
	m, err := loadModel()
	if err != nil {
		t.Fatal(err)
	}
	n, bad := 0, 0
	readJSONL(t, "testdata/samples.jsonl", func(line []byte) {
		var rec struct {
			Sample sample  `json:"sample"`
			Proba  float64 `json:"proba"`
		}
		if err := json.Unmarshal(line, &rec); err != nil {
			t.Fatal(err)
		}
		n++
		if got := m.proba(&rec.Sample); math.Abs(got-rec.Proba) > 1e-9 {
			if bad++; bad <= 10 {
				t.Errorf("%s: Go=%.12f Python=%.12f", rec.Sample.Name, got, rec.Proba)
			}
		}
	})
	t.Logf("примеров: %d, расхождений: %d", n, bad)
}

// Нормализатор: на тех же исходниках результат совпадает с normalize.py.
func TestNormalizeParity(t *testing.T) {
	n, bad := 0, 0
	readJSONL(t, "testdata/normalize.jsonl", func(line []byte) {
		var rec struct {
			File   string `json:"file"`
			Start  int    `json:"start"`
			End    int    `json:"end"`
			Sample sample `json:"sample"`
		}
		if err := json.Unmarshal(line, &rec); err != nil {
			t.Fatal(err)
		}
		src, err := os.ReadFile(rec.File)
		if err != nil {
			t.Fatal(err)
		}
		n++
		got, err := normalize(src, extension(rec.File), rec.Start, rec.End)
		if err == nil && reflect.DeepEqual(*got, rec.Sample) {
			return
		}
		if bad++; bad <= 10 {
			t.Errorf("%s:%d-%d\n  Go:     %+v (err=%v)\n  Python: %+v",
				rec.File, rec.Start, rec.End, got, err, rec.Sample)
		}
	})
	t.Logf("функций: %d, расхождений: %d", n, bad)
}
