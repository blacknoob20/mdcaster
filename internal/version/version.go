// Package version expone la versión del proyecto.
//
// Es la única fuente de verdad del número de SemVer: los dos binarios
// (`mdcaster` y `mdclean`) lo leen de aquí para su flag `--version`.
//
// La publica el agente al cerrar una release, según la política de
// `ARCHITECTURE.md` (D21) y el procedimiento de `AGENTS.md`.
package version

// Version es la versión actual del proyecto, en SemVer (MAJOR.MINOR.PATCH).
//
// Solo se edita este valor al publicar una release. Ningún otro fichero
// copia el número: las specs y el README lo describen de forma genérica
// para que no envejezcan en cada release.
const Version = "1.1.0"
