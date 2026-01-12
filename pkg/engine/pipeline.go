// Package engine provides the pipeline execution logic for GoFlow.
// It implements a functional pipeline approach using Go generics for type safety.
package engine

import (
	"context"
	"fmt"
	"sync"
)

// Link represents a single step in a pipeline.
// It's a function that takes an input of type I and returns an output of type O.
type Link[I, O any] func(ctx context.Context, input I) (O, error)

// Pipeline represents a sequence of processing steps.
// Each step transforms input data through a series of Links.
type Pipeline[I, O any] struct {
	name  string
	links []any // Stored as any to support heterogeneous link types
}

// NewPipeline creates a new pipeline with the given name.
func NewPipeline[I, O any](name string) *Pipeline[I, O] {
	return &Pipeline[I, O]{
		name:  name,
		links: make([]any, 0),
	}
}

// Chain connects two links together, creating a new link that pipes output to input.
// The output type of the first link must match the input type of the second.
func Chain[I, M, O any](first Link[I, M], second Link[M, O]) Link[I, O] {
	return func(ctx context.Context, input I) (O, error) {
		var zero O

		// Check for context cancellation
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
		}

		// Execute first link
		middle, err := first(ctx, input)
		if err != nil {
			return zero, fmt.Errorf("first link failed: %w", err)
		}

		// Check for context cancellation between links
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
		}

		// Execute second link
		output, err := second(ctx, middle)
		if err != nil {
			return zero, fmt.Errorf("second link failed: %w", err)
		}

		return output, nil
	}
}

// ParallelResult holds the result of a parallel execution.
type ParallelResult[O any] struct {
	Index  int
	Output O
	Err    error
}

// Parallel executes multiple links concurrently with the same input.
// Returns results in the same order as the input links.
// All links receive the same input and execute in separate goroutines.
func Parallel[I, O any](ctx context.Context, input I, links ...Link[I, O]) ([]O, error) {
	if len(links) == 0 {
		return nil, nil
	}

	results := make([]ParallelResult[O], len(links))
	var wg sync.WaitGroup

	// Create a cancellable context for coordinated shutdown
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Channel to collect the first error
	errChan := make(chan error, 1)

	for i, link := range links {
		wg.Add(1)
		go func(idx int, l Link[I, O]) {
			defer wg.Done()

			output, err := l(ctx, input)
			results[idx] = ParallelResult[O]{
				Index:  idx,
				Output: output,
				Err:    err,
			}

			if err != nil {
				select {
				case errChan <- err:
					cancel() // Cancel other goroutines on first error
				default:
				}
			}
		}(i, link)
	}

	wg.Wait()

	// Collect outputs and check for errors
	outputs := make([]O, len(links))
	for i, result := range results {
		if result.Err != nil {
			return nil, fmt.Errorf("parallel link %d failed: %w", i, result.Err)
		}
		outputs[i] = result.Output
	}

	return outputs, nil
}

// FanOut distributes input to multiple links and collects all results.
// Unlike Parallel, this continues even if some links fail, collecting all errors.
type FanOutResult[O any] struct {
	Outputs []O
	Errors  []error
}

// FanOut executes multiple links concurrently, collecting all results and errors.
// Does not fail fast - waits for all links to complete.
func FanOut[I, O any](ctx context.Context, input I, links ...Link[I, O]) FanOutResult[O] {
	if len(links) == 0 {
		return FanOutResult[O]{}
	}

	type indexedResult struct {
		index  int
		output O
		err    error
	}

	resultChan := make(chan indexedResult, len(links))
	var wg sync.WaitGroup

	for i, link := range links {
		wg.Add(1)
		go func(idx int, l Link[I, O]) {
			defer wg.Done()
			output, err := l(ctx, input)
			resultChan <- indexedResult{index: idx, output: output, err: err}
		}(i, link)
	}

	// Close channel after all goroutines complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results maintaining order
	outputs := make([]O, len(links))
	errors := make([]error, 0)

	for result := range resultChan {
		if result.err != nil {
			errors = append(errors, fmt.Errorf("link %d: %w", result.index, result.err))
		} else {
			outputs[result.index] = result.output
		}
	}

	return FanOutResult[O]{
		Outputs: outputs,
		Errors:  errors,
	}
}

// Map applies a link to each element of an input slice concurrently.
func Map[I, O any](ctx context.Context, inputs []I, link Link[I, O]) ([]O, error) {
	if len(inputs) == 0 {
		return nil, nil
	}

	results := make([]O, len(inputs))
	var mu sync.Mutex
	var firstErr error
	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for i, input := range inputs {
		wg.Add(1)
		go func(idx int, in I) {
			defer wg.Done()

			output, err := link(ctx, in)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("map index %d: %w", idx, err)
					cancel()
				}
				mu.Unlock()
				return
			}

			mu.Lock()
			results[idx] = output
			mu.Unlock()
		}(i, input)
	}

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	return results, nil
}

// Reduce combines multiple inputs into a single output using a reducer link.
type Reducer[I, O any] func(ctx context.Context, accumulator O, input I) (O, error)

// Reduce applies a reducer function sequentially to combine inputs.
func Reduce[I, O any](ctx context.Context, inputs []I, initial O, reducer Reducer[I, O]) (O, error) {
	accumulator := initial

	for i, input := range inputs {
		select {
		case <-ctx.Done():
			return accumulator, ctx.Err()
		default:
		}

		var err error
		accumulator, err = reducer(ctx, accumulator, input)
		if err != nil {
			return accumulator, fmt.Errorf("reduce step %d: %w", i, err)
		}
	}

	return accumulator, nil
}

// Retry wraps a link with retry logic.
func Retry[I, O any](link Link[I, O], maxAttempts int) Link[I, O] {
	return func(ctx context.Context, input I) (O, error) {
		var lastErr error
		var zero O

		for attempt := 0; attempt < maxAttempts; attempt++ {
			select {
			case <-ctx.Done():
				return zero, ctx.Err()
			default:
			}

			output, err := link(ctx, input)
			if err == nil {
				return output, nil
			}
			lastErr = err
		}

		return zero, fmt.Errorf("failed after %d attempts: %w", maxAttempts, lastErr)
	}
}
