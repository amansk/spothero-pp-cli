// Copyright 2026 Amandeep Khurana and contributors. Licensed under Apache-2.0. See LICENSE.
package main

import (
	"os"

	"github.com/amansk/spothero-pp-cli/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
