// sysinfo_influxdb by Novaquark
//
// To the extent possible under law, the person who associated CC0 with
// sysinfo_influxdb has waived all copyright and related or neighboring rights
// to sysinfo_influxdb.
//
// You should have received a copy of the CC0 legalcode along with this
// work.  If not, see <http://creativecommons.org/publicdomain/zero/1.0/>.

package main

import (
	"fmt"
	influxClient "github.com/influxdb/influxdb/client"
	"math/rand"
	"testing"
)

func TestSimple(t *testing.T) {
	serie1 := &influxClient.Point{
		Measurement:   "test_init",
		Tags:   map[string]string{"tag0": "val0"},
		Fields: map[string]interface{}{"col0": 42},
	}

	if DiffFromLast(serie1) != nil {
		t.Error("First time, initialization can't return a valid serie")
	}

	if serie1.Measurement != "test_init" {
		t.Error("DiffFromLast shouldn't modify serie name")
	}

	if len(serie1.Tags) != 1 {
		t.Error("DiffFromLast shouldn't modify tag number")
	}

	if _, ok := serie1.Tags["tag0"]; !ok {
		t.Error("DiffFromLast shouldn't modify tag name")
	}

	if v := serie1.Tags["tag0"]; v != "val0" {
		t.Error("DiffFromLast shouldn't modify tag value")
	}

	serie2 := &influxClient.Point{
		Measurement:   "test_init_other",
		Tags:   map[string]string{"tag0": "val0", "tag1": "val1"},
		Fields: map[string]interface{}{"col1": 22, "col0": 23},
	}

	if DiffFromLast(serie2) != nil {
		t.Error("Another serie (different serie name and tags) have to be initialized too")
	}

	serie1.Fields = map[string]interface{}{"col0": 23}
	serie2.Fields = map[string]interface{}{"col0": 43, "col1": 42}

	if DiffFromLast(serie1) == nil || DiffFromLast(serie2) == nil {
		t.Error("Initialized diff serie shouldn't return nil")
	}

	if serie1.Fields["col0"] != 23-42 {
		t.Error("Bad diff:", serie1.Fields["col0"], "!= 23 - 42")
	}

	if serie2.Fields["col0"] != 43-23 {
		t.Error("Bad diff:", serie2.Fields["col0"], "!= 43 - 23")
	}
	if serie2.Fields["col1"] != 42-22 {
		t.Error("Bad diff:", serie2.Fields["col1"], "!= 42 - 22")
	}
}

func fillPoints(serie *influxClient.Point, pts *map[string]interface{}, size int) {
	*pts = map[string]interface{}{}
	for i := 0; i < size; i++ {
		tmp := rand.Intn(9876543210)
		k := fmt.Sprint("col", i)
		(*pts)[k] = tmp
		serie.Fields[k] = tmp
	}
}

func TestRandom(t *testing.T) {
	serie := &influxClient.Point{
		Measurement:   fmt.Sprintf("test_rnd"),
		Tags:   map[string]string{"toto": "titi"},
		Fields: map[string]interface{}{},
	}

	size := rand.Intn(30) + 12

	var oldPts, newPts *map[string]interface{}
	newPts = new(map[string]interface{})

	fillPoints(serie, newPts, size)

	DiffFromLast(serie)

	for h := 0; h < rand.Intn(50)+10; h++ {
		oldPts = newPts
		newPts = new(map[string]interface{})

		fillPoints(serie, newPts, size)
		DiffFromLast(serie)

		// Compare
		for i := 0; i < size; i++ {
			k := fmt.Sprint("col", i)
			if serie.Fields[k] != (*newPts)[k].(int)-(*oldPts)[k].(int) {
				t.Error(fmt.Sprintf("Iteration %d; point %d: expected %d, got %d", h, k,
					(*newPts)[k].(int)-(*oldPts)[k].(int), serie.Fields[k]))
			}
		}
	}
}
