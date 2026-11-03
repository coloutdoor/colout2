package takeoff

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

var (
	estimateIDLine = regexp.MustCompile(`^Estimate ID: (\S+)`)
	deckLine       = regexp.MustCompile(`^Deck: [\d.]+ sq ft of (.+), ([\d.]+) ft high`)
	sectionLine    = regexp.MustCompile(`^\s*Section "(.*)": ([\d.]+) ft \(projection\) x ([\d.]+) ft \(width\)`)
	stairsLine     = regexp.MustCompile(`^Stairs: ([\d.]+) ft wide`)
	fasciaLine     = regexp.MustCompile(`^Fascia: ([\d.]+) lineal ft`)
)

// ParseEstimateText reads the estimate text written by formatEstimateText.
func ParseEstimateText(r io.Reader) (Input, error) {
	var in Input
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if m := estimateIDLine.FindStringSubmatch(line); m != nil {
			in.EstimateID = m[1]
		} else if m := deckLine.FindStringSubmatch(line); m != nil {
			height, err := strconv.ParseFloat(m[2], 64)
			if err != nil {
				return Input{}, fmt.Errorf("deck height %q: %w", m[2], err)
			}
			in.Material = m[1]
			in.HeightFt = height
		} else if m := sectionLine.FindStringSubmatch(line); m != nil {
			projection, err := strconv.ParseFloat(m[2], 64)
			if err != nil {
				return Input{}, fmt.Errorf("section projection %q: %w", m[2], err)
			}
			width, err := strconv.ParseFloat(m[3], 64)
			if err != nil {
				return Input{}, fmt.Errorf("section width %q: %w", m[3], err)
			}
			in.Sections = append(in.Sections, Section{Label: m[1], ProjectionFt: projection, WidthFt: width})
		} else if m := stairsLine.FindStringSubmatch(line); m != nil {
			stairWidth, err := strconv.ParseFloat(m[1], 64)
			if err != nil {
				return Input{}, fmt.Errorf("stair width %q: %w", m[1], err)
			}
			in.StairWidthFt = stairWidth
		} else if m := fasciaLine.FindStringSubmatch(line); m != nil {
			fasciaLF, err := strconv.ParseFloat(m[1], 64)
			if err != nil {
				return Input{}, fmt.Errorf("fascia length %q: %w", m[1], err)
			}
			in.FasciaLF = fasciaLF
		} else if strings.HasPrefix(line, "Stair Fascia: included") {
			in.StairFascia = true
		} else if strings.HasPrefix(line, "Stair Toe Kicks: included") {
			in.ToeKicks = true
		}
	}
	if err := scanner.Err(); err != nil {
		return Input{}, err
	}
	if in.EstimateID == "" {
		return Input{}, errors.New("no Estimate ID line found")
	}
	if in.Material == "" {
		return Input{}, errors.New("no Deck line found")
	}
	return in, nil
}
