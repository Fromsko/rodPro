// This file contains all query related code for Page and Element to separate the concerns.

package rod

import (
	"context"
	"errors"
	"regexp"

	"github.com/go-rod/rod/lib/cdp"
	"github.com/go-rod/rod/lib/js"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/rod/lib/utils"
)

// SelectorType enum
type SelectorType string

const (
	// SelectorTypeRegex type
	SelectorTypeRegex SelectorType = "regex"
	// SelectorTypeCSSSector type
	SelectorTypeCSSSector SelectorType = "css-selector"
	// SelectorTypeText type
	SelectorTypeText SelectorType = "text"
)

// Elements provides some helpers to deal with element list
type Elements []*Element

// First returns the first element, if the list is empty returns nil
func (els Elements) First() *Element {
	if els.Empty() {
		return nil
	}
	return els[0]
}

// Last returns the last element, if the list is empty returns nil
func (els Elements) Last() *Element {
	if els.Empty() {
		return nil
	}
	return els[len(els)-1]
}

// Empty returns true if the list is empty
func (els Elements) Empty() bool {
	return len(els) == 0
}

// Pages provides some helpers to deal with page list
type Pages []*Page

// First returns the first page, if the list is empty returns nil
func (ps Pages) First() *Page {
	if ps.Empty() {
		return nil
	}
	return ps[0]
}

// Last returns the last page, if the list is empty returns nil
func (ps Pages) Last() *Page {
	if ps.Empty() {
		return nil
	}
	return ps[len(ps)-1]
}

// Empty returns true if the list is empty
func (ps Pages) Empty() bool {
	return len(ps) == 0
}

// Find the page that has the specified element with the css selector
func (ps Pages) Find(selector string) (*Page, error) {
	for _, page := range ps {
		has, _, err := page.Has(selector)
		if err != nil {
			return nil, err
		}
		if has {
			return page, nil
		}
	}
	return nil, &ErrPageNotFound{}
}

// FindByURL returns the page that has the url that matches the jsRegex
func (ps Pages) FindByURL(jsRegex string) (*Page, error) {
	for _, page := range ps {
		res, err := page.Eval(`() => location.href`)
		if err != nil {
			return nil, err
		}
		url := res.Value.String()
		if regexp.MustCompile(jsRegex).MatchString(url) {
			return page, nil
		}
	}
	return nil, &ErrPageNotFound{}
}

// Has an element that matches the css selector
func (p *Page) Has(selector string) (bool, *Element, error) {
	el, err := p.Sleeper(NotFoundSleeper).Element(selector)
	if errors.Is(err, &ErrElementNotFound{}) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}
	return true, el.Sleeper(p.sleeper), nil
}

// HasX an element that matches the XPath selector
func (p *Page) HasX(selector string) (bool, *Element, error) {
	el, err := p.Sleeper(NotFoundSleeper).ElementX(selector)
	if errors.Is(err, &ErrElementNotFound{}) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}
	return true, el.Sleeper(p.sleeper), nil
}

// HasR an element that matches the css selector and its display text matches the jsRegex.
func (p *Page) HasR(selector, jsRegex string) (bool, *Element, error) {
	el, err := p.Sleeper(NotFoundSleeper).ElementR(selector, jsRegex)
	if errors.Is(err, &ErrElementNotFound{}) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}
	return true, el.Sleeper(p.sleeper), nil
}

// Element retries until an element in the page that matches the CSS selector, then returns
// the matched element.
func (p *Page) Element(selector string) (*Element, error) {
	return p.ElementByJS(evalHelper(js.Element, selector))
}

// ElementR retries until an element in the page that matches the css selector and it's text matches the jsRegex,
// then returns the matched element.
func (p *Page) ElementR(selector, jsRegex string) (*Element, error) {
	return p.ElementByJS(evalHelper(js.ElementR, selector, jsRegex))
}

// ElementX retries until an element in the page that matches one of the XPath selectors, then returns
// the matched element.
func (p *Page) ElementX(xPath string) (*Element, error) {
	return p.ElementByJS(evalHelper(js.ElementX, xPath))
}

// ElementByJS returns the element from the return value of the js function.
// If sleeper is nil, no retry will be performed.
// By default, it will retry until the js function doesn't return null.
// To customize the retry logic, check the examples of Page.Sleeper.
func (p *Page) ElementByJS(opts *EvalOptions) (*Element, error) {
	var res *proto.RuntimeRemoteObject
	var err error

	removeTrace := func() {}
	err = utils.Retry(p.ctx, p.sleeper(), func() (bool, error) {
		remove := p.tryTraceQuery(opts)
		removeTrace()
		removeTrace = remove

		res, err = p.Evaluate(opts.ByObject())
		if err != nil {
			return true, err
		}

		if res.Type == proto.RuntimeRemoteObjectTypeObject && res.Subtype == proto.RuntimeRemoteObjectSubtypeNull {
			return false, nil
		}

		return true, nil
	})
	removeTrace()
	if err != nil {
		return nil, err
	}

	if res.Subtype != proto.RuntimeRemoteObjectSubtypeNode {
		return nil, &ErrExpectElement{res}
	}

	return p.ElementFromObject(res)
}

// Elements returns all elements that match the css selector
func (p *Page) Elements(selector string) (Elements, error) {
	return p.ElementsByJS(evalHelper(js.Elements, selector))
}

// ElementsX returns all elements that match the XPath selector
func (p *Page) ElementsX(xpath string) (Elements, error) {
	return p.ElementsByJS(evalHelper(js.ElementsX, xpath))
}

// ElementsByJS returns the elements from the return value of the js
func (p *Page) ElementsByJS(opts *EvalOptions) (Elements, error) {
	res, err := p.Evaluate(opts.ByObject())
	if err != nil {
		return nil, err
	}

	if res.Subtype != proto.RuntimeRemoteObjectSubtypeArray {
		return nil, &ErrExpectElements{res}
	}

	defer func() { err = p.Release(res) }()

	list, err := proto.RuntimeGetProperties{
		ObjectID:      res.ObjectID,
		OwnProperties: true,
	}.Call(p)
	if err != nil {
		return nil, err
	}

	elemList := Elements{}
	for _, obj := range list.Result {
		if obj.Name == "__proto__" || obj.Name == "length" {
			continue
		}
		val := obj.Value

		if val.Subtype != proto.RuntimeRemoteObjectSubtypeNode {
			return nil, &ErrExpectElements{val}
		}

		el, err := p.ElementFromObject(val)
		if err != nil {
			return nil, err
		}

		elemList = append(elemList, el)
	}

	return elemList, err
}

// RetryOptions defines the configuration for the retry mechanism.
type RetryOptions struct {
	Context    context.Context             // The context to control the retry process.
	Sleeper    func(context.Context) error // Sleeper function to wait between retries.
	MaxRetries int                         // Maximum number of retries.
}

// NewRetry implements a retry mechanism based on the provided RetryOptions.
// The function `fn` is executed up to MaxRetries times until it indicates to stop or an error occurs.
func NewRetry(options RetryOptions, fn func() (stop bool, err error)) error {
	for i := 0; i < options.MaxRetries; i++ {
		stop, err := fn()
		if stop {
			return err
		}
		// Use the Sleeper function from options to wait before the next retry.
		err = options.Sleeper(options.Context)
		if err != nil {
			return err
		}
	}
	return nil // Return nil if the maximum retries are reached without success.
}

// Search performs a query in the DOM tree of the page.
// It retries the search until the result count is not zero or the maximum retries are reached.
// The query can be in the form of plain text, CSS selector, or XPath. It also searches within nested iframes and shadow DOMs.
func (p *Page) Search(query string) (*SearchResult, error) {
	sr := &SearchResult{
		page:    p,
		restore: p.EnableDomain(proto.DOMEnable{}),
	}

	// Configure the retry options for the search.
	retryOptions := RetryOptions{
		Context:    p.ctx,
		Sleeper:    p.sleeper(),
		MaxRetries: 3,
	}

	// Use the NewRetry function with the defined options and search logic.
	err := NewRetry(retryOptions, func() (bool, error) {
		if sr.DOMPerformSearchResult != nil {
			// Discard previous search results before performing a new search.
			_ = proto.DOMDiscardSearchResults{SearchID: sr.SearchID}.Call(p)
		}

		// Perform the search with the given query.
		res, err := proto.DOMPerformSearch{
			Query:                     query,
			IncludeUserAgentShadowDOM: true,
		}.Call(p)
		if err != nil {
			return true, err
		}

		sr.DOMPerformSearchResult = res

		// If no results are found, prepare for a retry.
		if res.ResultCount == 0 {
			return false, nil
		}

		// Retrieve the search results.
		result, err := proto.DOMGetSearchResults{
			SearchID:  res.SearchID,
			FromIndex: 0,
			ToIndex:   1,
		}.Call(p)
		if err != nil {
			// Handle specific errors related to search session.
			if errors.Is(err, cdp.ErrCtxNotFound) ||
				errors.Is(err, cdp.ErrSearchSessionNotFound) {
				return false, nil
			}
			return true, err
		}

		id := result.NodeIds[0]

		// Handle the case where the node ID is zero due to DOMDocumentUpdated event.
		// This requires a call to DOMGetDocument to reset the browser's tracker.
		if id == 0 {
			_, _ = proto.DOMGetDocument{}.Call(p)
			return false, nil
		}

		// Convert the NodeID to an Element.
		el, err := p.ElementFromNode(&proto.DOMNode{NodeID: id})
		if err != nil {
			return true, err
		}

		sr.First = el

		return true, nil
	})

	if err != nil {
		return nil, err
	}

	return sr, nil
}

// SearchResult handler
type SearchResult struct {
	*proto.DOMPerformSearchResult

	page    *Page
	restore func()

	// First element in the search result
	First *Element
}

// Get l elements at the index of i from the remote search result.
func (s *SearchResult) Get(i, l int) (Elements, error) {
	result, err := proto.DOMGetSearchResults{
		SearchID:  s.SearchID,
		FromIndex: i,
		ToIndex:   i + l,
	}.Call(s.page)
	if err != nil {
		return nil, err
	}

	list := Elements{}

	for _, id := range result.NodeIds {
		el, err := s.page.ElementFromNode(&proto.DOMNode{NodeID: id})
		if err != nil {
			return nil, err
		}
		list = append(list, el)
	}

	return list, nil
}

// All returns all elements
func (s *SearchResult) All() (Elements, error) {
	return s.Get(0, s.ResultCount)
}

// Release the remote search result
func (s *SearchResult) Release() {
	s.restore()
	_ = proto.DOMDiscardSearchResults{SearchID: s.SearchID}.Call(s.page)
}

type raceBranch struct {
	condition func(*Page) (*Element, error)
	callback  func(*Element) error
}

// RaceContext stores the branches to race
type RaceContext struct {
	page     *Page
	branches []*raceBranch
}

// Race creates a context to race selectors
func (p *Page) Race() *RaceContext {
	return &RaceContext{page: p}
}

// ElementFunc takes a custom function to determine race success
func (rc *RaceContext) ElementFunc(fn func(*Page) (*Element, error)) *RaceContext {
	rc.branches = append(rc.branches, &raceBranch{
		condition: fn,
	})
	return rc
}

// Element is similar to [Page.Element]
func (rc *RaceContext) Element(selector string) *RaceContext {
	return rc.ElementFunc(func(p *Page) (*Element, error) {
		return p.Element(selector)
	})
}

// ElementX is similar to [Page.ElementX]
func (rc *RaceContext) ElementX(selector string) *RaceContext {
	return rc.ElementFunc(func(p *Page) (*Element, error) {
		return p.ElementX(selector)
	})
}

// ElementR is similar to [Page.ElementR]
func (rc *RaceContext) ElementR(selector, regex string) *RaceContext {
	return rc.ElementFunc(func(p *Page) (*Element, error) {
		return p.ElementR(selector, regex)
	})
}

// ElementByJS is similar to [Page.ElementByJS]
func (rc *RaceContext) ElementByJS(opts *EvalOptions) *RaceContext {
	return rc.ElementFunc(func(p *Page) (*Element, error) {
		return p.ElementByJS(opts)
	})
}

// Search is similar to [Page.Search]
func (rc *RaceContext) Search(query string) *RaceContext {
	return rc.ElementFunc(func(p *Page) (*Element, error) {
		res, err := p.Search(query)
		if err != nil {
			return nil, err
		}
		res.Release()
		return res.First, nil
	})
}

// Handle adds a callback function to the most recent chained selector.
// The callback function is run, if the corresponding selector is
// present first, in the Race condition.
func (rc *RaceContext) Handle(callback func(*Element) error) *RaceContext {
	rc.branches[len(rc.branches)-1].callback = callback
	return rc
}

// Do the race
func (rc *RaceContext) Do() (*Element, error) {
	var el *Element
	err := utils.Retry(rc.page.ctx, rc.page.sleeper(), func() (stop bool, err error) {
		for _, branch := range rc.branches {
			bEl, err := branch.condition(rc.page.Sleeper(NotFoundSleeper))
			if err == nil {
				el = bEl.Sleeper(rc.page.sleeper)

				if branch.callback != nil {
					err = branch.callback(el)
				}
				return true, err
			} else if !errors.Is(err, &ErrElementNotFound{}) {
				return true, err
			}
		}
		return
	})
	return el, err
}

// Has an element that matches the css selector
func (el *Element) Has(selector string) (bool, *Element, error) {
	el, err := el.Element(selector)
	if errors.Is(err, &ErrElementNotFound{}) {
		return false, nil, nil
	}
	return err == nil, el, err
}

// HasX an element that matches the XPath selector
func (el *Element) HasX(selector string) (bool, *Element, error) {
	el, err := el.ElementX(selector)
	if errors.Is(err, &ErrElementNotFound{}) {
		return false, nil, nil
	}
	return err == nil, el, err
}

// HasR returns true if a child element that matches the css selector and its text matches the jsRegex.
func (el *Element) HasR(selector, jsRegex string) (bool, *Element, error) {
	el, err := el.ElementR(selector, jsRegex)
	if errors.Is(err, &ErrElementNotFound{}) {
		return false, nil, nil
	}
	return err == nil, el, err
}

// Element returns the first child that matches the css selector
func (el *Element) Element(selector string) (*Element, error) {
	return el.ElementByJS(evalHelper(js.Element, selector))
}

// ElementR returns the first child element that matches the css selector and its text matches the jsRegex.
func (el *Element) ElementR(selector, jsRegex string) (*Element, error) {
	return el.ElementByJS(evalHelper(js.ElementR, selector, jsRegex))
}

// ElementX returns the first child that matches the XPath selector
func (el *Element) ElementX(xPath string) (*Element, error) {
	return el.ElementByJS(evalHelper(js.ElementX, xPath))
}

// ElementByJS returns the element from the return value of the js
func (el *Element) ElementByJS(opts *EvalOptions) (*Element, error) {
	e, err := el.page.Context(el.ctx).Sleeper(NotFoundSleeper).ElementByJS(opts.This(el.Object))
	if err != nil {
		return nil, err
	}
	return e.Sleeper(el.sleeper), nil
}

// Parent returns the parent element in the DOM tree
func (el *Element) Parent() (*Element, error) {
	return el.ElementByJS(Eval(`() => this.parentElement`))
}

// Parents that match the selector
func (el *Element) Parents(selector string) (Elements, error) {
	return el.ElementsByJS(evalHelper(js.Parents, selector))
}

// Next returns the next sibling element in the DOM tree
func (el *Element) Next() (*Element, error) {
	return el.ElementByJS(Eval(`() => this.nextElementSibling`))
}

// Previous returns the previous sibling element in the DOM tree
func (el *Element) Previous() (*Element, error) {
	return el.ElementByJS(Eval(`() => this.previousElementSibling`))
}

// Elements returns all elements that match the css selector
func (el *Element) Elements(selector string) (Elements, error) {
	return el.ElementsByJS(evalHelper(js.Elements, selector))
}

// ElementsX returns all elements that match the XPath selector
func (el *Element) ElementsX(xpath string) (Elements, error) {
	return el.ElementsByJS(evalHelper(js.ElementsX, xpath))
}

// ElementsByJS returns the elements from the return value of the js
func (el *Element) ElementsByJS(opts *EvalOptions) (Elements, error) {
	return el.page.Context(el.ctx).ElementsByJS(opts.This(el.Object))
}
