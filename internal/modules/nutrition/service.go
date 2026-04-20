package nutrition

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	openai "github.com/sashabaranov/go-openai"

	"github.com/kerhoff/healthbot/internal/db/queries"
	"github.com/kerhoff/healthbot/internal/vm"
)

type Service struct {
	pool     *pgxpool.Pool
	vmClient *vm.Client
	ai       *openai.Client
	tgBot    *tgbotapi.BotAPI
}

func NewService(pool *pgxpool.Pool, vmClient *vm.Client, aiClient *openai.Client, tgBot *tgbotapi.BotAPI) *Service {
	return &Service{pool: pool, vmClient: vmClient, ai: aiClient, tgBot: tgBot}
}

func (s *Service) AnalyzePhoto(ctx context.Context, userID int64, fileID, mealType string) (*queries.MealLog, error) {
	// Download photo from Telegram
	fileURL, err := s.tgBot.GetFileDirectURL(fileID)
	if err != nil {
		return nil, fmt.Errorf("get file url: %w", err)
	}

	resp, err := http.Get(fileURL) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("download photo: %w", err)
	}
	defer resp.Body.Close()

	imgBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read photo: %w", err)
	}

	imageBase64 := base64.StdEncoding.EncodeToString(imgBytes)

	estimate, err := AnalyzeMealPhoto(ctx, s.ai, imageBase64)
	if err != nil {
		return nil, fmt.Errorf("analyze photo: %w", err)
	}

	cal := estimate.Calories
	protein := estimate.ProteinG
	carbs := estimate.CarbsG
	fat := estimate.FatG
	fid := fileID

	ml, err := queries.InsertMealLog(ctx, s.pool, &queries.MealLog{
		UserID:        userID,
		MealType:      mealType,
		PhotoFileID:   &fid,
		Calories:      &cal,
		ProteinG:      &protein,
		CarbsG:        &carbs,
		FatG:          &fat,
		AIRawResponse: estimate.RawJSON,
		Confirmed:     false,
	})
	if err != nil {
		return nil, fmt.Errorf("insert meal log: %w", err)
	}
	return ml, nil
}

func (s *Service) LogManual(ctx context.Context, userID int64, data map[string]any) (*queries.MealLog, error) {
	mealType, _ := data["meal_type"].(string)
	cal, _ := data["calories"].(int)
	protein, _ := data["protein"].(float64)
	carbs, _ := data["carbs"].(float64)
	fat, _ := data["fat"].(float64)

	ml, err := queries.InsertMealLog(ctx, s.pool, &queries.MealLog{
		UserID:    userID,
		MealType:  mealType,
		Calories:  &cal,
		ProteinG:  &protein,
		CarbsG:    &carbs,
		FatG:      &fat,
		Confirmed: true,
	})
	if err != nil {
		return nil, err
	}

	go func() {
		_ = s.vmClient.PushMeal(context.Background(), ml.MealType,
			*ml.Calories, *ml.ProteinG, *ml.CarbsG, *ml.FatG)
	}()

	return ml, nil
}

func (s *Service) ConfirmMeal(ctx context.Context, logID, userID int64) error {
	if err := queries.ConfirmMealLog(ctx, s.pool, logID, userID); err != nil {
		return err
	}
	ml, err := queries.GetMealLog(ctx, s.pool, logID, userID)
	if err != nil {
		return nil
	}
	if ml.Calories != nil && ml.ProteinG != nil {
		go func() {
			_ = s.vmClient.PushMeal(context.Background(), ml.MealType,
				*ml.Calories, *ml.ProteinG, *ml.CarbsG, *ml.FatG)
		}()
	}
	return nil
}

func (s *Service) DiscardMeal(ctx context.Context, logID, userID int64) error {
	return queries.DeleteMealLog(ctx, s.pool, logID, userID)
}

func (s *Service) UpdateMacros(ctx context.Context, logID, userID int64, cal int, protein, carbs, fat float64) error {
	return queries.UpdateMealLogMacros(ctx, s.pool, logID, userID, cal, protein, carbs, fat)
}

func (s *Service) TodaySummary(ctx context.Context, userID int64) (string, error) {
	meals, err := queries.GetTodayMeals(ctx, s.pool, userID)
	if err != nil {
		return "", err
	}
	if len(meals) == 0 {
		return "📋 No meals logged today.", nil
	}

	var totalCal int
	var totalProt, totalCarbs, totalFat float64
	var sb strings.Builder
	sb.WriteString("📋 Today's meals:\n\n")

	for _, m := range meals {
		cal, prot, carbs, fat := 0, 0.0, 0.0, 0.0
		if m.Calories != nil {
			cal = *m.Calories
		}
		if m.ProteinG != nil {
			prot = *m.ProteinG
		}
		if m.CarbsG != nil {
			carbs = *m.CarbsG
		}
		if m.FatG != nil {
			fat = *m.FatG
		}
		totalCal += cal
		totalProt += prot
		totalCarbs += carbs
		totalFat += fat
		sb.WriteString(fmt.Sprintf("• %s — %d kcal | P:%.0fg C:%.0fg F:%.0fg\n",
			m.MealType, cal, prot, carbs, fat))
	}
	sb.WriteString(fmt.Sprintf("\n📊 Total: %d kcal | P:%.0fg C:%.0fg F:%.0fg",
		totalCal, totalProt, totalCarbs, totalFat))
	return sb.String(), nil
}

func (s *Service) GetMealLog(ctx context.Context, logID, userID int64) (*queries.MealLog, error) {
	return queries.GetMealLog(ctx, s.pool, logID, userID)
}

func ParseInt(s string) (int, bool) {
	s = strings.TrimSpace(s)
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 || v > 10000 {
		return 0, false
	}
	return v, true
}

func ParseFloat(s string) (float64, bool) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 || v > 2000 {
		return 0, false
	}
	return v, true
}

// ErrNotFound is returned when a record is not found.
var ErrNotFound = pgx.ErrNoRows

func IsNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
