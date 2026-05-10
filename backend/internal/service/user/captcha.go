package user

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go-service-template/internal/errorz"
	"go-service-template/internal/service/sudoku"

	"github.com/google/uuid"
)

type CaptchaResult struct {
	Correct bool
	Errors  int
	Failed  bool
}

func (s *service) GenerateSudokuCaptcha(ctx context.Context, userID uuid.UUID) (uuid.UUID, [][]int, error) {
	puzzle, solution := sudoku.Generate()

	puzzleJSON, _ := json.Marshal(puzzle)
	solutionJSON, _ := json.Marshal(solution)

	expiresAt := time.Now().Add(10 * time.Minute)

	captcha, err := s.repo.CreateSudokuCaptcha(ctx, userID, puzzleJSON, solutionJSON, expiresAt)
	if err != nil {
		return uuid.Nil, nil, fmt.Errorf("failed to create sudoku captcha: %w", err)
	}

	puzzleSlice := make([][]int, 9)
	for i := 0; i < 9; i++ {
		puzzleSlice[i] = make([]int, 9)
		for j := 0; j < 9; j++ {
			puzzleSlice[i][j] = puzzle[i][j]
		}
	}

	return captcha.ID, puzzleSlice, nil
}

func (s *service) VerifySudokuCell(ctx context.Context, captchaID uuid.UUID, userID uuid.UUID, row, col, value int) (*CaptchaResult, error) {
	captcha, err := s.repo.GetSudokuCaptcha(ctx, captchaID)
	if err != nil {
		return nil, errorz.ErrCaptchaNotFound
	}

	if captcha.UserID != userID {
		return nil, errorz.ErrCaptchaForbidden
	}

	if captcha.Passed {
		return nil, errorz.ErrCaptchaGone
	}

	var solution [9][9]int
	_ = json.Unmarshal(captcha.Solution, &solution)

	if solution[row][col] == value {
		return &CaptchaResult{
			Correct: true,
			Errors:  int(captcha.Errors),
			Failed:  false,
		}, nil
	}

	updated, err := s.repo.IncrementSudokuErrors(ctx, captchaID)
	if err != nil {
		return nil, err
	}

	if updated.Errors > 3 {
		// Penalty: 10 minutes cooldown
		cooldownUntil := time.Now().Add(10 * time.Minute)
		_ = s.repo.SetCaptchaCooldown(ctx, userID, cooldownUntil)
		_ = s.repo.DeleteSudokuCaptcha(ctx, captchaID)
		return &CaptchaResult{
			Correct: false,
			Errors:  int(updated.Errors),
			Failed:  true,
		}, nil
	}

	return &CaptchaResult{
		Correct: false,
		Errors:  int(updated.Errors),
		Failed:  false,
	}, nil
}

func (s *service) CompleteSudoku(ctx context.Context, captchaID uuid.UUID, userID uuid.UUID) (string, error) {
	captcha, err := s.repo.GetSudokuCaptcha(ctx, captchaID)
	if err != nil {
		return "", errorz.ErrCaptchaNotFound
	}

	if captcha.UserID != userID {
		return "", errorz.ErrCaptchaForbidden
	}

	if captcha.Errors > 3 {
		return "", errorz.ErrCaptchaForbidden
	}

	_, err = s.repo.MarkSudokuPassed(ctx, captchaID)
	if err != nil {
		return "", err
	}

	// Generate a captcha token
	token, err := s.jwtManager.GenerateCaptchaToken(userID)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *service) GenerateCasinoCaptcha(ctx context.Context, userID uuid.UUID) (uuid.UUID, time.Time, error) {
	expiresAt := time.Now().Add(10 * time.Minute)

	captcha, err := s.repo.CreateCasinoCaptcha(ctx, userID, expiresAt)
	if err != nil {
		return uuid.Nil, time.Time{}, fmt.Errorf("failed to create casino captcha: %w", err)
	}

	return captcha.ID, captcha.ExpiresAt.Time, nil
}

type CasinoSpinResult struct {
	Win           bool
	CaptchaToken  string
	CooldownUntil *time.Time
}

func (s *service) SpinCasino(ctx context.Context, captchaID uuid.UUID, userID uuid.UUID) (*CasinoSpinResult, error) {
	captcha, err := s.repo.GetCasinoCaptcha(ctx, captchaID)
	if err != nil {
		return nil, errorz.ErrCaptchaNotFound
	}

	if captcha.UserID != userID {
		return nil, errorz.ErrCaptchaForbidden
	}

	if captcha.Passed {
		return nil, errorz.ErrCaptchaGone
	}

	if captcha.ExpiresAt.Time.Before(time.Now()) {
		return nil, errorz.ErrCaptchaGone
	}

	// 1 in 4 chance to win
	win := s.rng.IntN(4) == 0

	if win {
		_, err = s.repo.MarkCasinoPassed(ctx, captchaID)
		if err != nil {
			return nil, err
		}

		token, err := s.jwtManager.GenerateCaptchaToken(userID)
		if err != nil {
			return nil, err
		}

		return &CasinoSpinResult{
			Win:          true,
			CaptchaToken: token,
		}, nil
	}

	// Loss: 10 minutes cooldown
	cooldownUntil := time.Now().Add(10 * time.Minute)
	_ = s.repo.SetCaptchaCooldown(ctx, userID, cooldownUntil)
	_ = s.repo.DeleteCasinoCaptcha(ctx, captchaID)

	return &CasinoSpinResult{
		Win:           false,
		CooldownUntil: &cooldownUntil,
	}, nil
}
