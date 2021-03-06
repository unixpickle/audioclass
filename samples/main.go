package main

import (
	"flag"
	"fmt"
	"math/rand"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/unixpickle/audioset"
	"github.com/unixpickle/essentials"
	"github.com/unixpickle/speechrecog/mfcc"
)

const inSampleRate = 22050

func main() {
	rand.Seed(time.Now().UnixNano())

	var csvPath string
	var wavDir string
	var align int
	var stride int
	var augment bool
	var useMFCC bool

	flag.StringVar(&csvPath, "csv", "", "path to segment CSV file")
	flag.StringVar(&wavDir, "dir", "", "path to sample download directory")
	flag.IntVar(&align, "align", 512, "PCM sample count alignment")
	flag.IntVar(&stride, "stride", 1, "PCM sample stride for downsampling")
	flag.BoolVar(&augment, "augment", false, "perform data augmentation")
	flag.BoolVar(&useMFCC, "mfcc", false, "use MFCC output instead of PCM")
	flag.Parse()

	if csvPath == "" || wavDir == "" {
		essentials.Die("Required flags: -csv and -dir. See -help.")
	}

	samples, err := audioset.ReadSet(wavDir, csvPath)
	if err != nil {
		essentials.Die(err)
	}

	classes := samples.Classes()

	sampleChan := loopedSamples(samples)
	lineChan := make(chan string, 1)
	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		go func() {
			for sample := range sampleChan {
				data, err := sample.Read()
				if err != nil {
					essentials.Die(err)
				}
				if !useMFCC {
					data = downsample(data, stride)
				}
				if augment {
					data = audioset.Augment(data)
				}
				classStr := classesToStr(classes, sample)
				if useMFCC {
					coeffs := mfccStream(data)
					lineChan <- floatsToStr(coeffs) + "\n" + classStr
				} else {
					if len(data)%align != 0 {
						padding := make([]float64, align-(len(data)%align))
						data = append(data, padding...)
					}
					lineChan <- floatsToStr(data) + "\n" + classStr
				}
			}
		}()
	}

	for line := range lineChan {
		fmt.Println(line)
	}
}

func mfccStream(data []float64) []float64 {
	source := &mfcc.SliceSource{Slice: data}
	outSource := mfcc.MFCC(source, inSampleRate, nil)
	var res []float64
	for {
		next, err := outSource.NextCoeffs()
		if err != nil {
			break
		}
		res = append(res, next...)
	}
	return res
}

func loopedSamples(samples audioset.Set) <-chan *audioset.Sample {
	res := make(chan *audioset.Sample)
	go func() {
		for {
			perm := rand.Perm(len(samples))
			for _, i := range perm {
				sample := samples[i]
				res <- sample
			}
		}
	}()
	return res
}

func floatsToStr(data []float64) string {
	var parts []string
	for _, x := range data {
		parts = append(parts, strconv.FormatFloat(x, 'f', -1, 32))
	}
	return strings.Join(parts, " ")
}

func classesToStr(classes []string, sample *audioset.Sample) string {
	var vec []string
	for _, class := range classes {
		var present bool
		for _, x := range sample.Classes {
			if x == class {
				present = true
				break
			}
		}
		if present {
			vec = append(vec, "1")
		} else {
			vec = append(vec, "0")
		}
	}
	return strings.Join(vec, " ")
}

func downsample(data []float64, stride int) []float64 {
	if stride == 1 {
		return data
	}
	res := make([]float64, (len(data)+(stride-1))/stride)
	for i := range res {
		res[i] = data[i*stride]
	}
	return res
}
