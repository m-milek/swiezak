package report

import (
	"log/slog"

	"github.com/m-milek/swiezak/internal/check"
	"github.com/m-milek/swiezak/internal/docker"
)

func Print(img docker.Image, result check.Result) {
	_, _, pin := img.Reference()
	slog.Info("result", "image", img.ConfigImage, "pinned", pin != "", "status", result.Status, "major", result.Major, "minor", result.Minor, "patch", result.Patch)
}
