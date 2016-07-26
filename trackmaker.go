package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fohristiwhirl/wavmaker"
)

var midi_freq [128]float64 = [128]float64{
	8.175799, 8.661957, 9.177024, 9.722718, 10.300861, 10.913382, 11.562326, 12.249857,
	12.978272, 13.750000, 14.567618, 15.433853, 16.351598, 17.323914, 18.354048, 19.445436,
	20.601722, 21.826764, 23.124651, 24.499715, 25.956544, 27.500000, 29.135235, 30.867706,
	32.703196, 34.647829, 36.708096, 38.890873, 41.203445, 43.653529, 46.249303, 48.999429,
	51.913087, 55.000000, 58.270470, 61.735413, 65.406391, 69.295658, 73.416192, 77.781746,
	82.406889, 87.307058, 92.498606, 97.998859, 103.826174, 110.000000, 116.540940, 123.470825,
	130.812783, 138.591315, 146.832384, 155.563492, 164.813778, 174.614116, 184.997211, 195.997718,
	207.652349, 220.000000, 233.081881, 246.941651, 261.625565, 277.182631, 293.664768, 311.126984,
	329.627557, 349.228231, 369.994423, 391.995436, 415.304698, 440.000000, 466.163762, 493.883301,
	523.251131, 554.365262, 587.329536, 622.253967, 659.255114, 698.456463, 739.988845, 783.990872,
	830.609395, 880.000000, 932.327523, 987.766603, 1046.502261, 1108.730524, 1174.659072, 1244.507935,
	1318.510228, 1396.912926, 1479.977691, 1567.981744, 1661.218790, 1760.000000, 1864.655046, 1975.533205,
	2093.004522, 2217.461048, 2349.318143, 2489.015870, 2637.020455, 2793.825851, 2959.955382, 3135.963488,
	3322.437581, 3520.000000, 3729.310092, 3951.066410, 4186.009045, 4434.922096, 4698.636287, 4978.031740,
	5274.040911, 5587.651703, 5919.910763, 6271.926976, 6644.875161, 7040.000000, 7458.620184, 7902.132820,
	8372.018090, 8869.844191, 9397.272573, 9956.063479, 10548.081821, 11175.303406, 11839.821527, 12543.853951,
}

type Instrument struct {
	notes [128]*wavmaker.WAV
	flags [128]bool
	ready bool
}

type ParserState struct {
	line uint32					// current line in score
	instrument_name string
	volume float64
	position uint32				// in samples
	jump uint32
}

var instruments = make(map[string]*Instrument)
var default_instrument_name string


// ---------------------------------------------------------- METHODS


func (instrument *Instrument) addfile(notestring string, filename string) error {

	note, err := name_to_midi(notestring)
	if err != nil {
		return err
	}

	wav, err := wavmaker.Load(filename)

	if err != nil {
		return err
	}

	instrument.notes[note] = wav
	instrument.flags[note] = true
	instrument.ready = true

	return nil
}


// ---------------------------------------------------------- FUNCTIONS


func main() {

	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s directory\n", filepath.Base(os.Args[0]))
		os.Exit(1)
	}
	err := os.Chdir(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	load_instruments("instruments.txt")
	score_to_wav("score.txt")
}


func load_instruments(filename string) {

	var scanner *bufio.Scanner

	instruments_file, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't read %s\n", filename)
		os.Exit(1)
	}
	defer instruments_file.Close()

	scanner = bufio.NewScanner(instruments_file)

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 3 {

			insname, notename, filename := fields[0], fields[1], fields[2]

			// Format is:    piano G4 piano.ff.G4.wav

			if default_instrument_name == "" {
				default_instrument_name = insname
			}

			ins, ok := instruments[insname]
			if ok == false {
				ins = new(Instrument)
				instruments[insname] = ins
			}

			err = ins.addfile(notename, filename)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Couldn't add %s to %s: %v\n", filename, insname, err)
			}
		}
	}
}


func score_to_wav(filename string) {

	output, score_file := initial_score_load(filename)
	defer score_file.Close()

	parser_state := initial_parser_state()
	scanner := bufio.NewScanner(score_file)

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		handle_score_line(&parser_state, fields, output)
	}

	// Fadeout and save...

	output.FadeSamples(44100)
	output.Save("trackmaker_output.wav")
}


func initial_parser_state() ParserState {
	var s = ParserState{
			line : 0,
			instrument_name : default_instrument_name,
			volume : 1.0,
			position : 0,
			jump : 11025,
	}
	return s
}


func initial_score_load(filename string) (*wavmaker.WAV, *os.File) {

	// Create a wav file with the correct size for the score, and
	// return a pointer to it as well as a pointer to the score file.

	// Load file...

	score_file, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't read %s\n", filename)
		os.Exit(1)
	}

	// Determine expected WAV length by parsing score and updating a state struct...

	parser_state := initial_parser_state()
	scanner := bufio.NewScanner(score_file)

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		handle_score_line(&parser_state, fields, nil)
	}

	// Create output wav, and return it, along with the score file

	output_wav := wavmaker.New(parser_state.position)

	score_file.Seek(0, 0)		// Important: reset score file position
	return output_wav, score_file
}


func handle_score_line(settings *ParserState, fields []string, output_wav *wavmaker.WAV) {

	// TODO: the meat of the score parser. The score can, conceptually,
	// change the settings (e.g. the instrument name, volume, speed).

	for _, token := range fields {

		_, err := name_to_midi(token)	// FIXME: using this function just for its err is crude
		if err != nil {

			// instrument name? ----------------------------------------------------------------

			_, ok := instruments[token]
			if ok {
				settings.instrument_name = token
				continue
			}

			// jump setting? (i.e. frames between notes) ---------------------------------------

			if strings.HasPrefix(token, "j:") {
				j, err := strconv.Atoi(token[2:])
				if err != nil {
					fmt.Fprintf(os.Stderr, "line %d: bad token \"%s\"\n", settings.line, token)
				} else {
					settings.jump = uint32(j)
				}
				continue
			}

			// We didn't figure out what the token means ---------------------------------------

			fmt.Fprintf(os.Stderr, "line %d: unknown token \"%s\"\n", settings.line, token)

		} else {

			// The token is a note...

			if output_wav != nil {
				err = insert_by_name(settings.instrument_name, token, output_wav, settings.position)
				if err != nil {
					fmt.Printf("line %d: %v\n", settings.line, err)
				}
			}
		}
	}

	settings.line += 1
	settings.position += settings.jump
}


func insert_by_name(instrument_name string, notename string, target_wav *wavmaker.WAV, t_loc uint32) error {

	// Get the named instrument from the global instruments map,
	// and insert it into the wav with the given note, creating
	// that note if needed...

	i, ok := instruments[instrument_name]
	if ok == false {
		return fmt.Errorf("insert_by_name() couldn't find instrument \"%s\"", instrument_name)
	}

	if i.ready == false {
		return fmt.Errorf("insert_by_name() called on an empty instrument")
	}

	note, err := name_to_midi(notename)		// A number between 0 and 127 (MIDI value corresponding to note)
	if err != nil {
		return fmt.Errorf("insert_by_name(): %v", err)
	}

	if i.notes[note] == nil {

		a := int(note)
		b := int(note)

		note_to_stretch := 0

		for {		// Find reference note (one with its flag set) which was loaded from a file
			a--
			b++

			if a >= 0 {
				if i.flags[a] {
					note_to_stretch = a
					break
				}
			}

			if b <= 127 {
				if i.flags[b] {
					note_to_stretch = b
					break
				}
			}

			if a <= 0 && b >= 127 {
				return fmt.Errorf("insert() couldn't find a reference note")	// Should be impossible
			}
		}

		ins_freq := midi_freq[note]
		ref_freq := midi_freq[note_to_stretch]

		i.notes[note] = i.notes[note_to_stretch].StretchedRelative(ref_freq / ins_freq)
	}

	target_wav.Add(t_loc, i.notes[note], 0, i.notes[note].FrameCount())
	return nil
}


func name_to_midi(name string) (int, error) {

	// Accepts notes in the following formats: C4  C4#  C#4  C4b  Cb4

	var result, number, accidental int
	var letter string

	if len(name) == 2 {
		letter = string(name[0])
		letter = strings.ToUpper(letter)
		number = int(name[1]) - 48				// -48 is conversion of ASCII to int
	} else if len(name) == 3 {
		letter = string(name[0])
		letter = strings.ToUpper(letter)
		if name[1] == '#' || name[1] == 'b' {
			number = int(name[2]) - 48
			if name[1] == '#' {
				accidental = 1
			} else {
				accidental = -1
			}
		} else if name[2] == '#' || name[2] == 'b' {
			number = int(name[1]) - 48
			if name[2] == '#' {
				accidental = 1
			} else {
				accidental = -1
			}
		} else {
			return 0, fmt.Errorf("name_to_midi(%s): string format was wrong", name)
		}
	} else {
		return 0, fmt.Errorf("name_to_midi(%s): string length was wrong", name)
	}

	// First we set the result as if we asked for C in the relevant octave...

	switch number {
		case 0: result = 12		// C0
		case 1: result = 24		// C1
		case 2:	result = 36		// C2
		case 3:	result = 48		// C3
		case 4:	result = 60		// C4
		case 5:	result = 72		// C5
		case 6:	result = 84		// C6
		case 7:	result = 96		// C7
		case 8:	result = 108	// C8
		case 9: result = 120	// C9
		default: return 0, fmt.Errorf("name_to_midi(%s): note number was wrong", name)
	}

	// Now we adjust it for the actual note that was requested...

	switch letter {
		case "C": result += 0
		case "D": result += 2
		case "E": result += 4
		case "F": result += 5
		case "G": result += 7
		case "A": result += 9
		case "B": result += 11
		default: return 0, fmt.Errorf("name_to_midi(%s): note letter was wrong", name)
	}

	// Now take into account flat or sharp symbols...

	result += accidental

	if result < 0 || result > 127 {
		return 0, fmt.Errorf("name_to_midi(%s): resulting note out of range 0-127", name)
	}

	return result, nil
}
