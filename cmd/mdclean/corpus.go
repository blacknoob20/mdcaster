// Datos del corpus institucional.
//
// Aquí viven las listas que son específicas de un tipo de documento: las
// cabeceras que se repiten en cada página, las imágenes de cabecera y los
// marcadores de sección. Generalizar la limpieza institucional es tocar este
// fichero, no el resto del comando.
package main

import "regexp"

// Cabeceras institucionales que se repiten en cada página del export.
// La comparación normaliza mayúsculas y espacios.
var tdrHeaderLines = []string{
	"GAD MUNICIPAL DE MACHALA",
	"DIRECCIÓN DE TECNOLOGÍA DE LA INFORMACIÓN",
	"COORDINACIÓN DE DESARROLLO DE SISTEMAS",
	"COORDINACIÓN DE DESARROLLO DE SISTEMAS INFORMÁTICOS",
}

// Imágenes de cabecera que se repiten en cada página del export.
var tdrHeaderImages = []string{
	"media/rId40.png",
	"media/rId43.png",
	"media/rId46.png",
	"media/rId49.jpg",
	"media/rId52.png",
	"media/rId55.png",
}

// reSeccionTDR marca las secciones que, en el modo TDR, se convierten en
// encabezado de nivel 2.
var reSeccionTDR = regexp.MustCompile(`^(GOBIERNO|GAD|MUNICIPAL|DIRECCIÓN|COORDINACIÓN|SECCIÓN|OBJETO|NOMBRE|CONTRATACIÓN|TÉRMINOS|ANTECEDENTES|JUSTIFICACION|OBJETIVOS|ALCANCE|METODOLOGÍA|INFORMACIÓN|PRODUCTOS|PLAZO|PERSONAL|FORMA|CONDICIONES|ANEXO)`)

// Marcadores de sección del corpus institucional. Una línea que empieza por
// uno de ellos no se une con la línea siguiente.
var (
	reNotaGeneral           = regexp.MustCompile(`^NOTA GENERAL:$`)
	reTituloCapituloSeccion = regexp.MustCompile(`^(TITULO|Capítulo|Sección)\b`)
	reArticulo              = regexp.MustCompile(`^Art\.`)
	reConcordancias         = regexp.MustCompile(`^(Concordancias|Jurisprudencia):$`)
	reInstitucional         = regexp.MustCompile(`^(FUNCION EJECUTIVA|ASAMBLEA NACIONAL|PRESIDENCIA DE LA REPUBLICA)\b`)
	reElPleno               = regexp.MustCompile(`^EL PLENO$`)
	reConsiderando          = regexp.MustCompile(`^CONSIDERANDO:$`)
)

// structuralLines agrupa los marcadores de sección del corpus institucional.
var structuralLines = []*regexp.Regexp{
	reNotaGeneral,
	reTituloCapituloSeccion,
	reArticulo,
	reConcordancias,
	reInstitucional,
	reElPleno,
	reConsiderando,
}
