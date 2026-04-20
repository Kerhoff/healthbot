package stats

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

// Chart rendering using go-echarts generates HTML. We use a simple
// text-based PNG approach for Telegram since go-echarts renders HTML
// which requires a headless browser for PNG conversion.
// Instead, we generate PNG charts using a simple SVG-to-PNG approach
// with a pure Go SVG renderer.

// RenderWeightChart renders a simple text-based weight chart as a PNG-encoded byte slice.
// Due to the pure-Go constraint (no CGo/headless browser), we render as SVG bytes
// which Telegram can display as a document, or we use a simple bar/line ASCII fallback.
// For now, we render SVG which Telegram can display.
func RenderWeightChart(dates []time.Time, weights []float64, title string) ([]byte, error) {
	if len(dates) == 0 {
		return nil, fmt.Errorf("no data")
	}
	return renderLineSVG(title, "Weight (kg)", dates, weights, "#4A90D9")
}

func RenderFastingChart(dates []time.Time, hours []float64, title string) ([]byte, error) {
	if len(dates) == 0 {
		return nil, fmt.Errorf("no data")
	}
	return renderBarSVG(title, "Hours", dates, hours, "#E8A838")
}

func RenderCaloriesChart(dates []time.Time, calories []float64, title string) ([]byte, error) {
	if len(dates) == 0 {
		return nil, fmt.Errorf("no data")
	}
	return renderBarSVG(title, "Calories", dates, calories, "#E85050")
}

const svgWidth = 600
const svgHeight = 300
const padLeft = 60
const padRight = 20
const padTop = 40
const padBottom = 50

func renderLineSVG(title, yLabel string, dates []time.Time, vals []float64, color string) ([]byte, error) {
	n := len(vals)
	minV, maxV := vals[0], vals[0]
	for _, v := range vals {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	if maxV == minV {
		maxV = minV + 1
	}

	chartW := svgWidth - padLeft - padRight
	chartH := svgHeight - padTop - padBottom

	xScale := float64(chartW) / float64(n-1)
	if n == 1 {
		xScale = 0
	}
	yScale := float64(chartH) / (maxV - minV)

	var points strings.Builder
	for i, v := range vals {
		x := padLeft + float64(i)*xScale
		y := float64(padTop+chartH) - (v-minV)*yScale
		if i == 0 {
			fmt.Fprintf(&points, "M %.1f,%.1f", x, y)
		} else {
			fmt.Fprintf(&points, " L %.1f,%.1f", x, y)
		}
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d">`, svgWidth, svgHeight)
	fmt.Fprintf(&buf, `<rect width="%d" height="%d" fill="#1e1e2e"/>`, svgWidth, svgHeight)
	fmt.Fprintf(&buf, `<text x="%d" y="25" font-family="Arial" font-size="14" fill="white" text-anchor="middle">%s</text>`,
		svgWidth/2, escXML(title))
	fmt.Fprintf(&buf, `<path d="%s" stroke="%s" stroke-width="2" fill="none"/>`, points.String(), color)

	// Y axis labels
	for i := 0; i <= 4; i++ {
		v := minV + float64(i)*(maxV-minV)/4
		y := float64(padTop+chartH) - float64(i)*float64(chartH)/4
		fmt.Fprintf(&buf, `<text x="%d" y="%.1f" font-family="Arial" font-size="10" fill="#aaa" text-anchor="end">%.1f</text>`,
			padLeft-5, y+4, v)
		fmt.Fprintf(&buf, `<line x1="%d" y1="%.1f" x2="%d" y2="%.1f" stroke="#333" stroke-width="1"/>`,
			padLeft, y, padLeft+chartW, y)
	}

	// X axis labels (show up to 6)
	step := n / 6
	if step < 1 {
		step = 1
	}
	for i := 0; i < n; i += step {
		x := padLeft + float64(i)*xScale
		fmt.Fprintf(&buf, `<text x="%.1f" y="%d" font-family="Arial" font-size="9" fill="#aaa" text-anchor="middle">%s</text>`,
			x, svgHeight-padBottom+20, dates[i].Format("01/02"))
	}

	// Data points
	for i, v := range vals {
		x := padLeft + float64(i)*xScale
		y := float64(padTop+chartH) - (v-minV)*yScale
		fmt.Fprintf(&buf, `<circle cx="%.1f" cy="%.1f" r="3" fill="%s"/>`, x, y, color)
	}

	fmt.Fprintf(&buf, `</svg>`)
	return buf.Bytes(), nil
}

func renderBarSVG(title, yLabel string, dates []time.Time, vals []float64, color string) ([]byte, error) {
	n := len(vals)
	maxV := 0.0
	for _, v := range vals {
		if v > maxV {
			maxV = v
		}
	}
	if maxV == 0 {
		maxV = 1
	}

	chartW := svgWidth - padLeft - padRight
	chartH := svgHeight - padTop - padBottom
	barW := float64(chartW)/float64(n) * 0.7
	spacing := float64(chartW) / float64(n)

	var buf bytes.Buffer
	fmt.Fprintf(&buf, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d">`, svgWidth, svgHeight)
	fmt.Fprintf(&buf, `<rect width="%d" height="%d" fill="#1e1e2e"/>`, svgWidth, svgHeight)
	fmt.Fprintf(&buf, `<text x="%d" y="25" font-family="Arial" font-size="14" fill="white" text-anchor="middle">%s</text>`,
		svgWidth/2, escXML(title))

	// Y axis
	for i := 0; i <= 4; i++ {
		v := float64(i) * maxV / 4
		y := float64(padTop+chartH) - float64(i)*float64(chartH)/4
		fmt.Fprintf(&buf, `<text x="%d" y="%.1f" font-family="Arial" font-size="10" fill="#aaa" text-anchor="end">%.0f</text>`,
			padLeft-5, y+4, v)
		fmt.Fprintf(&buf, `<line x1="%d" y1="%.1f" x2="%d" y2="%.1f" stroke="#333" stroke-width="1"/>`,
			padLeft, y, padLeft+chartW, y)
	}

	for i, v := range vals {
		x := float64(padLeft) + float64(i)*spacing + (spacing-barW)/2
		h := v / maxV * float64(chartH)
		y := float64(padTop+chartH) - h
		fmt.Fprintf(&buf, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="%s" rx="2"/>`,
			x, y, barW, h, color)
		// X label
		fmt.Fprintf(&buf, `<text x="%.1f" y="%d" font-family="Arial" font-size="9" fill="#aaa" text-anchor="middle">%s</text>`,
			x+barW/2, svgHeight-padBottom+20, dates[i].Format("01/02"))
	}

	fmt.Fprintf(&buf, `</svg>`)
	return buf.Bytes(), nil
}

func escXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

