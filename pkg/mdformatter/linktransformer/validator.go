// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package linktransformer

import (
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type Validator interface {
	IsValid(k futureKey, r *validator) (bool, error)
}

// GitHubPullsIssuesValidator.IsValid skips visiting all github issue/PR links.
func (v GitHubPullsIssuesValidator) IsValid(k futureKey, r *validator) (bool, error) {
	r.l.githubSkippedLinks.Inc()
	// Find rightmost index of match i.e, where regex match ends.
	// This will be where issue/PR number starts. Split incase of section link and convert to int.
	rightmostIndex := v._regex.FindStringIndex(k.dest)
	stringNum := strings.Split(k.dest[rightmostIndex[1]:], "#")
	num, err := strconv.Atoi(stringNum[0])
	if err != nil {
		return false, err
	}
	// If number in link does not exceed then link is valid.
	if v._maxNum >= num {
		return true, nil
	}
	return false, nil
}

// RoundTripValidator.IsValid returns true if url is checked by colly.
func (v RoundTripValidator) IsValid(k futureKey, r *validator) (bool, error) {
	r.l.roundTripLinks.Inc()
	// Result will be in future.
	r.destFutures[k].resultFn = func() error { return r.remoteLinks[k.dest] }
	r.rMu.RLock()
	if _, ok := r.remoteLinks[k.dest]; ok {
		r.rMu.RUnlock()
		return true, nil
	}
	r.rMu.RUnlock()

	r.rMu.Lock()
	defer r.rMu.Unlock()
	// We need to check again here to avoid race.
	if _, ok := r.remoteLinks[k.dest]; ok {
		return true, nil
	}

	r.c.WithTransport(r.transportFn(k.dest))
	if err := r.c.Visit(k.dest); err != nil {
		r.remoteLinks[k.dest] = errors.Wrapf(err, "remote link %v", k.dest)
		return false, nil
	}
	return true, nil
}

// IgnoreValidator.IsValid returns true if matched so that link in not checked.
func (v IgnoreValidator) IsValid(k futureKey, r *validator) (bool, error) {
	r.l.ignoreSkippedLinks.Inc()

	return true, nil
}

// GetValidatorForURL returns correct Validator by matching URL.
func (v Config) GetValidatorForURL(URL string) Validator {
	for _, val := range v.Validators {
		switch val.Type {
		case roundtripValidator:
			if !val.rtValidator._regex.MatchString(URL) {
				continue
			}
			return val.rtValidator
		case githubPullsIssuesValidator:
			if !val.ghValidator._regex.MatchString(URL) {
				continue
			}
			return val.ghValidator
		case ignoreValidator:
			if !val.igValidator._regex.MatchString(URL) {
				continue
			}
			return val.igValidator
		default:
			panic("unexpected validator type")
		}
	}
	// No config file passed, so all links must be checked.
	return RoundTripValidator{}
}
