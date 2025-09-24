package elastic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/tpelletiersophos/cloudcutter/internal/services/elastic"
)

type searchResult struct {
	entries   []*DocEntry
	totalHits int
}

func (v *View) fetchRegularResults(query map[string]any, numResults int, index string) (*searchResult, error) {
	query["size"] = numResults

	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		v.state.misc.rateLimit.Wait()

		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(query); err != nil {
			return nil, fmt.Errorf("error encoding query: %v", err)
		}

		res, err := v.service.Client.Search(
			v.service.Client.Search.WithIndex(index),
			v.service.Client.Search.WithBody(&buf),
			v.service.Client.Search.WithScroll(5*time.Minute),
		)

		if err != nil {
			lastErr = err
			if strings.Contains(err.Error(), "429") {
				v.state.misc.rateLimit.HandleTooManyRequests()
				v.manager.UpdateStatusBar(fmt.Sprintf("Rate limited (attempt %d/%d), retrying in %v...",
					attempt+1, maxRetries, v.state.misc.rateLimit.GetRetryAfter()))
				continue
			}
			return nil, fmt.Errorf("search error: %v", err)
		}
		defer res.Body.Close()

		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading response: %v", err)
		}

		// rate limit?
		if res.StatusCode == 429 {
			v.state.misc.rateLimit.HandleTooManyRequests()
			v.manager.UpdateStatusBar(fmt.Sprintf("Rate limited (attempt %d/%d), retrying in %v...",
				attempt+1, maxRetries, v.state.misc.rateLimit.GetRetryAfter()))
			continue
		}

		v.state.misc.rateLimit.Reset()

		var result elastic.ESSearchResult
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			return nil, fmt.Errorf("error decoding response: %v", err)
		}

		entries, err := v.processSearchResults(result.Hits.Hits)
		if err != nil {
			return nil, fmt.Errorf("error processing results: %v", err)
		}

		return &searchResult{
			entries:   entries,
			totalHits: result.Hits.GetTotalHits(),
		}, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %v", lastErr)
}

func (v *View) fetchLargeResults(query map[string]any, index string) (*searchResult, error) {
	query["size"] = 1000
	maxRetries := 3
	var lastErr error

	// Initial scroll request with retries
	var scrollID string
	var allResults []*DocEntry

	for attempt := 0; attempt < maxRetries; attempt++ {
		v.state.misc.rateLimit.Wait()

		queryJSON, err := json.Marshal(query)
		if err != nil {
			return nil, fmt.Errorf("error creating query: %v", err)
		}

		res, err := v.service.Client.Search(
			v.service.Client.Search.WithIndex(index),
			v.service.Client.Search.WithBody(strings.NewReader(string(queryJSON))),
			v.service.Client.Search.WithScroll(time.Duration(5)*time.Minute),
		)
		if err != nil {
			lastErr = err
			if strings.Contains(err.Error(), "429") {
				v.state.misc.rateLimit.HandleTooManyRequests()
				v.manager.UpdateStatusBar(fmt.Sprintf("Rate limited (attempt %d/%d), retrying in %v...",
					attempt+1, maxRetries, v.state.misc.rateLimit.GetRetryAfter()))
				continue
			}
			return nil, fmt.Errorf("initial scroll error: %v", err)
		}

		bodyBytes, err := io.ReadAll(res.Body)
		res.Body.Close()

		if err != nil {
			return nil, fmt.Errorf("error reading response: %v", err)
		}

		if res.StatusCode == 429 {
			v.state.misc.rateLimit.HandleTooManyRequests()
			v.manager.UpdateStatusBar(fmt.Sprintf("Rate limited (attempt %d/%d), retrying in %v...",
				attempt+1, maxRetries, v.state.misc.rateLimit.GetRetryAfter()))
			continue
		}

		var result elastic.ESSearchResult
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			return nil, fmt.Errorf("error decoding response: %v", err)
		}

		entries, err := v.processSearchResults(result.Hits.Hits)
		if err != nil {
			return nil, fmt.Errorf("error processing batch: %v", err)
		}
		allResults = append(allResults, entries...)
		scrollID = result.ScrollID

		// Successfully got first batch
		v.state.misc.rateLimit.Reset()
		break
	}

	if scrollID == "" {
		return nil, fmt.Errorf("max retries exceeded: %v", lastErr)
	}

	// Continue scrolling with retries
	for {
		var scrollResult elastic.ESSearchResult

		// Try to get next batch with retries
		for attempt := 0; attempt < maxRetries; attempt++ {
			v.state.misc.rateLimit.Wait()

			scrollRes, err := v.service.Client.Scroll(
				v.service.Client.Scroll.WithScrollID(scrollID),
				v.service.Client.Scroll.WithScroll(time.Duration(5)*time.Minute),
			)
			if err != nil {
				if strings.Contains(err.Error(), "429") {
					v.state.misc.rateLimit.HandleTooManyRequests()
					v.manager.UpdateStatusBar(fmt.Sprintf("Rate limited (attempt %d/%d), retrying in %v...",
						attempt+1, maxRetries, v.state.misc.rateLimit.GetRetryAfter()))
					continue
				}
				return nil, fmt.Errorf("scroll error: %v", err)
			}

			if scrollRes.StatusCode == 429 {
				scrollRes.Body.Close()
				v.state.misc.rateLimit.HandleTooManyRequests()
				v.manager.UpdateStatusBar(fmt.Sprintf("Rate limited (attempt %d/%d), retrying in %v...",
					attempt+1, maxRetries, v.state.misc.rateLimit.GetRetryAfter()))
				continue
			}

			bodyBytes, err := io.ReadAll(scrollRes.Body)
			scrollRes.Body.Close()

			if err != nil {
				return nil, fmt.Errorf("error reading scroll response: %v", err)
			}

			if err := json.Unmarshal(bodyBytes, &scrollResult); err != nil {
				return nil, fmt.Errorf("error decoding scroll response: %v", err)
			}

			v.state.misc.rateLimit.Reset()
			break
		}

		if len(scrollResult.Hits.Hits) == 0 {
			_, err := v.service.Client.ClearScroll(
				v.service.Client.ClearScroll.WithScrollID(scrollID),
			)
			if err != nil {
				v.manager.Logger().Error("Failed to clear scroll", "error", err)
			}
			break
		}

		entries, err := v.processSearchResults(scrollResult.Hits.Hits)
		if err != nil {
			return nil, fmt.Errorf("error processing batch: %v", err)
		}
		allResults = append(allResults, entries...)
	}

	return &searchResult{
		entries:   allResults,
		totalHits: len(allResults),
	}, nil
}

func (v *View) executeSearch(query map[string]any) (*elastic.ESSearchResult, error) {
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Wait according to rate limit
		v.state.misc.rateLimit.Wait()

		queryJSON, err := json.Marshal(query)
		if err != nil {
			v.manager.Logger().Error("Error marshaling query", "error", err)
			return nil, fmt.Errorf("error creating query: %v", err)
		}

		v.manager.Logger().Debug("Executing search query", "index", v.state.search.currentIndex, "query", string(queryJSON))

		res, err := v.service.Client.Search(
			v.service.Client.Search.WithIndex(v.state.search.currentIndex),
			v.service.Client.Search.WithBody(bytes.NewReader(queryJSON)),
		)
		if err != nil {
			lastErr = err
			// rate limit?
			if strings.Contains(err.Error(), "429") {
				v.state.misc.rateLimit.HandleTooManyRequests()
				v.manager.UpdateStatusBar(fmt.Sprintf("Rate limited, retrying in %v...", v.state.misc.rateLimit.GetRetryAfter()))
				continue
			}
			v.manager.Logger().Error("Search query failed", "error", err, "index", v.state.search.currentIndex)
			return nil, fmt.Errorf("search error: %v", err)
		}
		defer res.Body.Close()

		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			v.manager.Logger().Error("Failed to read search response", "error", err)
			return nil, fmt.Errorf("error reading response body: %v", err)
		}

		// rate limit?
		if res.StatusCode == 429 {
			v.state.misc.rateLimit.HandleTooManyRequests()
			v.manager.UpdateStatusBar(fmt.Sprintf("Rate limited, retrying in %v...", v.state.misc.rateLimit.GetRetryAfter()))
			continue
		}

		// Reset rate limit backoff on successful request
		v.state.misc.rateLimit.Reset()

		v.manager.Logger().Debug("Raw search response", "response", string(bodyBytes))

		var result elastic.ESSearchResult
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			v.manager.Logger().Error("Failed to unmarshal search response", "error", err, "response", string(bodyBytes))
			return nil, fmt.Errorf("error decoding response: %v", err)
		}

		v.manager.Logger().Info("Search executed successfully", "hits", result.Hits.Total.Value, "took", result.Took)
		return &result, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %v", lastErr)
}

func (v *View) fetchResults() (*searchResult, error) {
	v.state.mu.RLock()
	numResults := v.state.search.numResults
	query := v.buildQuery()
	currentIndex := v.state.search.currentIndex
	v.state.mu.RUnlock()

	if numResults > 10000 {
		return v.fetchLargeResults(query, currentIndex)
	}
	return v.fetchRegularResults(query, numResults, currentIndex)
}

func (v *View) refreshResults() {
	v.state.mu.Lock()
	if v.state.ui.isLoading {
		v.state.mu.Unlock()
		return
	}
	v.state.ui.isLoading = true
	v.state.mu.Unlock()

	currentFocus := v.manager.App().GetFocus()
	v.showLoading("Refreshing results")

	go func() {
		defer func() {
			v.state.mu.Lock()
			v.state.ui.isLoading = false
			v.state.mu.Unlock()
			v.hideLoading()
			v.manager.App().QueueUpdateDraw(func() {
				v.manager.SetFocus(currentFocus)
			})
		}()

		searchResult, err := v.fetchResults()
		if err != nil {
			v.manager.Logger().Error("Error fetching results", "error", err)
			v.manager.App().QueueUpdateDraw(func() {
				v.manager.UpdateStatusBar(fmt.Sprintf("Error: %v", err))
			})
			return
		}

		v.manager.Logger().Info("Results refreshed successfully",
			"totalResults", len(searchResult.entries))

		v.updateFieldsFromResults(searchResult.entries)

		// Update results state
		v.state.mu.Lock()
		v.state.data.filteredResults = searchResult.entries
		v.state.data.displayedResults = append([]*DocEntry(nil), searchResult.entries...)
		v.state.pagination.totalPages = int(math.Ceil(float64(len(searchResult.entries)) /
			float64(v.state.pagination.pageSize)))
		if v.state.pagination.totalPages < 1 {
			v.state.pagination.totalPages = 1
		}
		v.state.mu.Unlock()

		v.manager.App().QueueUpdateDraw(func() {
			//v.updateIndexStats()
			v.displayCurrentPage()
			v.updateHeader()
			v.manager.UpdateStatusBar(fmt.Sprintf("Found %d results total (displaying %d)",
				searchResult.totalHits, len(searchResult.entries)))
		})
	}()
}

func (v *View) processSearchResults(hits []elastic.ESSearchHit) ([]*DocEntry, error) {
	results := make([]*DocEntry, 0, len(hits))

	for _, hit := range hits {
		entry, err := NewDocEntry(
			hit.Source,
			hit.ID,
			hit.Index,
			hit.Type,
			hit.Score,
			hit.Version,
		)
		if err != nil {
			continue
		}
		results = append(results, entry)
	}

	return results, nil
}

func (v *View) buildQuery() map[string]any {
	v.state.mu.RLock()
	filters := make([]string, len(v.state.data.filters))
	copy(filters, v.state.data.filters)
	timeframe := v.state.search.timeframe
	numResults := v.state.search.numResults
	v.state.mu.RUnlock()

	query, err := BuildQuery(filters, numResults, timeframe, v.state.data.fieldCache)
	if err != nil {
		v.manager.Logger().Error("Error building query", "error", err)
		v.manager.UpdateStatusBar(fmt.Sprintf("Error building query: %v", err))
		return nil
	}
	return query
}
