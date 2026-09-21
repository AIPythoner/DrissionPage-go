// Package drissionpage provides Chromium automation and HTTP sessions with
// DrissionPage-style locators. This Go port follows the Python 5.0.0b1 source.
// Browser operations use the Chrome DevTools Protocol through Rod.
// All potentially blocking operations accept a context; no API uses Must/panic.
package drissionpage

const SourceVersion = "5.0.0b1"
