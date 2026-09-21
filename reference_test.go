package drissionpage

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestPythonStaticReference(t *testing.T) {
	data, err := os.ReadFile("testdata/reference.json")
	if err != nil {
		t.Fatal(err)
	}
	var reference struct {
		HTML     string
		Locators []struct {
			Locator string
			IDs     []string
		}
		Texts   []struct{ ID, Text string }
		Scalars []struct {
			Expression string
			Value      any
		}
	}
	if err = json.Unmarshal(data, &reference); err != nil {
		t.Fatal(err)
	}
	document, err := MakeSessionElement(reference.HTML)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range reference.Locators {
		t.Run(test.Locator, func(t *testing.T) {
			elements, err := document.Eles(test.Locator)
			if err != nil {
				t.Fatal(err)
			}
			ids := []string{}
			for _, element := range elements {
				value, _ := element.Attr("id")
				ids = append(ids, value)
			}
			if !reflect.DeepEqual(ids, test.IDs) {
				t.Fatalf("Go %q; Python %q", ids, test.IDs)
			}
		})
	}
	for _, test := range reference.Texts {
		t.Run("text_"+test.ID, func(t *testing.T) {
			element, err := document.Ele("#" + test.ID)
			if err != nil {
				t.Fatal(err)
			}
			if value := element.Text(); value != test.Text {
				t.Fatalf("Go %q; Python %q", value, test.Text)
			}
		})
	}
	for _, test := range reference.Scalars {
		t.Run(test.Expression, func(t *testing.T) {
			value, err := document.XPathValues(test.Expression)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(value, test.Value) {
				t.Fatalf("Go %#v; Python %#v", value, test.Value)
			}
		})
	}

}
