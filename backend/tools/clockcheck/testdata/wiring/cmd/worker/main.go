// A command may build the real clock: this is where it is wired.
package main

import "github.com/shrujancheruku/issho/backend/internal/clock"

func main() { _ = clock.Real{} }
