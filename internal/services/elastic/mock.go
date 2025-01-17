package elastic

import (
	"fmt"
	"net/http"
)

type MockESClient struct {
	GetFunc         func(index string, id string, o ...func(*http.Request)) (*http.Response, error)
	SearchFunc      func(o ...func(*http.Request)) (*http.Response, error)
	ClearScrollFunc func(o ...func(*http.Request)) (*http.Response, error)
	ScrollFunc      func(o ...func(*http.Request)) (*http.Response, error)
	CatIndicesFunc  func(o ...func(*http.Request)) (*http.Response, error)
}

func (m *MockESClient) Get(index string, id string, o ...func(*http.Request)) (*http.Response, error) {
	if m.GetFunc != nil {
		return m.GetFunc(index, id, o...)
	}
	return nil, fmt.Errorf("Get not implemented")
}

func (m *MockESClient) Search(o ...func(*http.Request)) (*http.Response, error) {
	if m.SearchFunc != nil {
		return m.SearchFunc(o...)
	}
	return nil, fmt.Errorf("Search not implemented")
}

func (m *MockESClient) ClearScroll(o ...func(*http.Request)) (*http.Response, error) {
	if m.ClearScrollFunc != nil {
		return m.ClearScrollFunc(o...)
	}
	return nil, fmt.Errorf("ClearScroll not implemented")
}

func (m *MockESClient) Scroll(o ...func(*http.Request)) (*http.Response, error) {
	if m.ScrollFunc != nil {
		return m.ScrollFunc(o...)
	}
	return nil, fmt.Errorf("Scroll not implemented")
}
