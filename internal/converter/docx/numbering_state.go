package docx

// numCounter es el contador de un nivel de una lista.
type numCounter struct {
	// next es el próximo valor que se emitirá.
	next int
	// initialized indica si el contador ya se ha usado.
	initialized bool
}

// numState guarda los contadores de todas las listas del documento, indexados
// por (numId, nivel).
//
// Los contadores no se reinician al cambiar de numId: Word mantiene una
// numeración continua por instancia de lista. Solo se reinician los niveles
// profundos cuando Word lo haría, según w:lvlRestart.
type numState struct {
	counters map[string][]numCounter
}

// newNumState crea el estado de contadores.
func newNumState() *numState {
	return &numState{counters: make(map[string][]numCounter)}
}

// take devuelve el valor del contador del nivel y lo avanza.
func (s *numState) take(numID string, ilvl, start int) int {
	cs := s.counters[numID]
	for len(cs) <= ilvl {
		cs = append(cs, numCounter{next: start})
	}

	c := &cs[ilvl]
	if !c.initialized {
		c.next = start
		c.initialized = true
	}

	value := c.next
	c.next++
	s.counters[numID] = cs

	return value
}

// peek devuelve el valor vigente del contador del nivel sin avanzarlo: el
// último emitido, o el inicial si todavía no se ha usado.
func (s *numState) peek(numID string, ilvl, start int) int {
	cs := s.counters[numID]
	if ilvl >= len(cs) || !cs[ilvl].initialized {
		return start
	}
	return cs[ilvl].next - 1
}

// reset devuelve el contador del nivel a su valor inicial.
func (s *numState) reset(numID string, ilvl, start int) {
	cs := s.counters[numID]
	for len(cs) <= ilvl {
		cs = append(cs, numCounter{next: start})
	}
	cs[ilvl].next = start
	cs[ilvl].initialized = true
	s.counters[numID] = cs
}

// resetDeeperLevels reinicia los niveles más profundos que el indicado.
func (n *docxNumbering) resetDeeperLevels(numID string, ilvl int, state *numState) {
	levels, ok := n.levels[numID]
	if !ok {
		return
	}

	for deeper := ilvl + 1; deeper < len(levels)+9; deeper++ {
		lvl, exists := levels[deeper]
		if !exists {
			continue
		}

		switch {
		case lvl.lvlRestart < 0:
			// Sin declarar: se reinicia cuando cualquier nivel más
			// superficial avanza.
			state.reset(numID, deeper, lvl.start)
		case lvl.lvlRestart == 0:
			// Nunca se reinicia.
		default:
			// Se reinicia cuando avanza el nivel lvlRestart-1.
			if ilvl == lvl.lvlRestart-1 {
				state.reset(numID, deeper, lvl.start)
			}
		}
	}
}
