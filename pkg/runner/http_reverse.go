/*
MIT License

Copyright (c) 2023 API Testing Authors.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package runner

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"

	"github.com/linuxsuren/api-testing/pkg/testing"
	"github.com/linuxsuren/api-testing/pkg/util"
)

type Mutator interface {
	Render(*testing.TestCase) *testing.TestCase
	Message() string
}

type authHeaderMissingMutator struct{}

func (m *authHeaderMissingMutator) Render(testcase *testing.TestCase) (result *testing.TestCase) {
	result = &testing.TestCase{}
	_ = DeepCopy(testcase, result)
	delete(result.Request.Header, util.Authorization)
	return
}

func (m *authHeaderMissingMutator) Message() string {
	return "Missing Authorization in header"
}

type authHeaderRandomMutator struct{}

func (m *authHeaderRandomMutator) Render(testcase *testing.TestCase) (result *testing.TestCase) {
	result = &testing.TestCase{}
	_ = DeepCopy(testcase, result)
	if result.Request.Header == nil {
		result.Request.Header = make(map[string]string)
	}
	result.Request.Header[util.Authorization] = util.String(6)
	return
}

func (m *authHeaderRandomMutator) Message() string {
	return "Random Authorization in header"
}

type requiredQueryMutator struct {
	field string
}

func (m *requiredQueryMutator) Render(testcase *testing.TestCase) (result *testing.TestCase) {
	result = &testing.TestCase{}
	_ = DeepCopy(testcase, result)
	delete(result.Request.Query, m.field)
	return
}

func (m *requiredQueryMutator) Message() string {
	return fmt.Sprintf("Missing required query field: %q", m.field)
}

type minLengthQueryMutator struct {
	field  string
	length int
}

func (m *minLengthQueryMutator) Render(testcase *testing.TestCase) (result *testing.TestCase) {
	result = &testing.TestCase{}
	_ = DeepCopy(testcase, result)
	if result.Request.Query != nil && m.length > 1 {
		result.Request.Query[m.field] = util.String(m.length - 1)
	}
	return
}

func (m *minLengthQueryMutator) Message() string {
	return fmt.Sprintf("Min length query field: %q", m.field)
}

func DeepCopy(src, dist interface{}) (err error) {
	buf := bytes.Buffer{}
	if err = gob.NewEncoder(&buf).Encode(src); err != nil {
		return
	}
	return gob.NewDecoder(&buf).Decode(dist)
}

type reverseHTTPRunner struct {
	TestCaseRunner
}

func NewReverseHTTPRunner(normal TestCaseRunner) TestCaseRunner {
	return &reverseHTTPRunner{
		TestCaseRunner: normal,
	}
}

func (r *reverseHTTPRunner) RunTestCase(testcase *testing.TestCase, dataContext interface{},
	ctx context.Context) (output interface{}, err error) {
	// find all the mutators

	var mutators []Mutator
	if _, ok := testcase.Request.Header[util.Authorization]; ok {
		mutators = append(mutators, &authHeaderMissingMutator{}, &authHeaderRandomMutator{})
	}

	for k := range testcase.Request.Query {
		verifier := testcase.Request.Query.GetVerifier(k)
		if verifier == nil {
			continue
		}

		if verifier.Required {
			mutators = append(mutators, &requiredQueryMutator{field: k})
		}
		if verifier.MinLength > 0 {
			mutators = append(mutators, &minLengthQueryMutator{
				field:  k,
				length: verifier.MinLength,
			})
		}
	}

	for _, mutator := range mutators {
		mutationCase := mutator.Render(testcase)
		_, reverseErr := r.TestCaseRunner.RunTestCase(mutationCase, dataContext, ctx)
		if reverseErr == nil {
			err = fmt.Errorf("failed when: %q", mutator.Message())
			return
		}
	}
	return
}
