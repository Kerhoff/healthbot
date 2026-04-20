package vm

import (
	"context"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

type Client struct {
	remoteWriteURL string
	userLabel      string
	registry       *prometheus.Registry
}

func NewClient(remoteWriteURL, userLabel string) *Client {
	return &Client{
		remoteWriteURL: remoteWriteURL,
		userLabel:      userLabel,
		registry:       prometheus.NewRegistry(),
	}
}

func (c *Client) push(ctx context.Context, metrics map[string]float64, labels prometheus.Labels) error {
	if c.remoteWriteURL == "" {
		return nil
	}
	labels["user"] = c.userLabel

	reg := prometheus.NewRegistry()
	for name, val := range metrics {
		g := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: name}, labelKeys(labels))
		reg.MustRegister(g)
		g.With(labels).Set(val)
	}

	pusher := push.New(c.remoteWriteURL, "healthbot").
		Gatherer(reg).
		Client(http.DefaultClient)

	if err := pusher.AddContext(ctx); err != nil {
		return fmt.Errorf("vm push: %w", err)
	}
	return nil
}

// PushWeight pushes a weight metric.
func (c *Client) PushWeight(ctx context.Context, weightKg float64) error {
	return c.push(ctx, map[string]float64{"healthbot_weight_kg": weightKg}, prometheus.Labels{})
}

// PushFast pushes a fasting duration metric.
func (c *Client) PushFast(ctx context.Context, durationHours float64) error {
	return c.push(ctx, map[string]float64{"healthbot_fast_duration_hours": durationHours}, prometheus.Labels{})
}

// PushMeal pushes meal nutrition metrics.
func (c *Client) PushMeal(ctx context.Context, mealType string, calories int, proteinG, carbsG, fatG float64) error {
	if c.remoteWriteURL == "" {
		return nil
	}
	labels := prometheus.Labels{"user": c.userLabel, "meal_type": mealType}

	reg := prometheus.NewRegistry()
	calC := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "healthbot_meal_calories_total"}, []string{"user", "meal_type"})
	protG := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "healthbot_meal_protein_g"}, []string{"user", "meal_type"})
	carbG := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "healthbot_meal_carbs_g"}, []string{"user", "meal_type"})
	fatGv := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "healthbot_meal_fat_g"}, []string{"user", "meal_type"})
	reg.MustRegister(calC, protG, carbG, fatGv)

	calC.With(labels).Add(float64(calories))
	protG.With(labels).Set(proteinG)
	carbG.With(labels).Set(carbsG)
	fatGv.With(labels).Set(fatG)

	return push.New(c.remoteWriteURL, "healthbot").Gatherer(reg).AddContext(ctx)
}

// PushMedication pushes a medication event.
func (c *Client) PushMedication(ctx context.Context, status string) error {
	if c.remoteWriteURL == "" {
		return nil
	}
	labels := prometheus.Labels{"user": c.userLabel, "status": status}
	reg := prometheus.NewRegistry()
	ctr := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "healthbot_medication_event_total"}, []string{"user", "status"})
	reg.MustRegister(ctr)
	ctr.With(labels).Inc()
	return push.New(c.remoteWriteURL, "healthbot").Gatherer(reg).AddContext(ctx)
}

// PushBodyMeasurement pushes a body measurement metric.
func (c *Client) PushBodyMeasurement(ctx context.Context, part string, valueCm float64) error {
	if c.remoteWriteURL == "" {
		return nil
	}
	labels := prometheus.Labels{"user": c.userLabel, "part": part}
	reg := prometheus.NewRegistry()
	g := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "healthbot_body_measurement_cm"}, []string{"user", "part"})
	reg.MustRegister(g)
	g.With(labels).Set(valueCm)
	return push.New(c.remoteWriteURL, "healthbot").Gatherer(reg).AddContext(ctx)
}

func labelKeys(labels prometheus.Labels) []string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	return keys
}

