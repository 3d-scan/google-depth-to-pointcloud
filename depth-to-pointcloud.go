package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io/ioutil"
	"os"
	"strings"
	"text/template"
)

//create well-formed XML struct for GDepth extraction
type XMP struct {
	XMLName xml.Name `xml:"xmpmeta"`
	RDF     RDFTag   `xml:"RDF"`
}
type RDFTag struct {
	XMLName     xml.Name   `xml:"RDF"`
	Description GDepthMeta `xml:"Description"`
}
type GDepthMeta struct {
	XMLName xml.Name `xml:"Description"`
	Format  string   `xml:"http://ns.google.com/photos/1.0/depthmap/ Format,attr"`
	Near    float64  `xml:"http://ns.google.com/photos/1.0/depthmap/ Near,attr"`
	Far     float64  `xml:"http://ns.google.com/photos/1.0/depthmap/ Far,attr"`
	Mime    string   `xml:"http://ns.google.com/photos/1.0/depthmap/ Mime,attr"`
	Data    string   `xml:"http://ns.google.com/photos/1.0/depthmap/ Data,attr"`
	//These attributes are standard but don't appear in Google Camera lensblur images
	/*Units          string   `xml:"http://ns.google.com/photos/1.0/depthmap/ Units,attr"`
	MeasureType    string   `xml:"http://ns.google.com/photos/1.0/depthmap/ MeasureType,attr"`
	ConfidenceMime string   `xml:"http://ns.google.com/photos/1.0/depthmap/ ConfidenceMime,attr"`
	Confidence     string   `xml:"http://ns.google.com/photos/1.0/depthmap/ Confidence,attr"`
	Manufacturer   string   `xml:"http://ns.google.com/photos/1.0/depthmap/ Manufacturer,attr"`
	Model          string   `xml:"http://ns.google.com/photos/1.0/depthmap/ Model,attr"`
	Software       string   `xml:"http://ns.google.com/photos/1.0/depthmap/ Software,attr"`
	ImageWidth     string   `xml:"http://ns.google.com/photos/1.0/depthmap/ ImageWidth,attr"`
	ImageHeight    string   `xml:"http://ns.google.com/photos/1.0/depthmap/ ImageHeight,attr"`*/
}

//Pointcloud Data Structure
type Point struct {
	X, Y    uint
	Z       float64
	R, G, B uint
}

//Template for PLY files
var ply_template = template.Must(template.ParseFiles("ply_template.txt"))

//for error checking
func check(e error) {
	if e != nil {
		panic(e)
	}
}

func gDepthReader(path string) (pic image.Image, depth image.Image, near float64, far float64) {
	//jpeg markers for segments
	FF := byte(255)
	APP1 := byte(225)
	//import jpeg as byte slice, set it as pic
	data, err := ioutil.ReadFile(path)
	check(err)
	pic, err = jpeg.Decode(bytes.NewReader(data))
	check(err)
	//initialize metadata struct
	meta := XMP{}
	//make metadata byte slice with capacity of original jpeg
	bite := make([]byte, 0, len(data))
	//initialize marker index, length
	var begin, leng uint
	var no2 bool

	//now loop through jpeg file until you find the SECOND APP1 marker, which contains XMP data
	//there must be a better way to do this, but i'm an idiot, thus, brute-force FTW

	for ; ; begin++ {
		if data[begin] == FF {
			if data[begin+1] == APP1 {
				//if this isn't the second marker, keep going
				if !no2 {
					no2 = true
					continue
				} else {
					//Herein lies dragons. The length of the APP1 segment, in bytes, equals the next two bytes, minus the namespace bytes for XMP.
					leng = 256*uint(data[begin+2]) + uint(data[begin+3]) - 31
					//Actual content for APP1 segment begins 33 bytes later
					begin += 33
					err = xml.Unmarshal(data[begin:begin+leng], &meta)
					check(err)
					break
				}
			}
		}
	}

	//New loop for new ridiculous jpeg parsing. jpeg can't handle segments larger than 64KB, so XMP
	//has something called "extended XMP," which is just a bunch of 64KB blocks.

	//So I'm going to find every other fucking APP1 marker and retrieve/append the rest of the data

	begin = begin + leng

	for ; begin < uint(len(data)); begin++ {
		if data[begin] == FF {
			if data[begin+1] == APP1 {
				//Herein lies bigger dragons.
				//The length of the ExtendedXMP APP1 segment, in bytes, equals the next two bytes, minus namespace, GUID, length and offset bytes
				leng = 256*uint(data[begin+2]) + uint(data[begin+3]) - 77
				//Actual content for ExtendedXMP APP1 segment begins 79 bytes later
				begin += 79
				//Append the data
				bite = append(bite, data[begin:begin+leng]...)
				//Move loop forward
				begin = begin + leng - 1
			}
		}
	}

	//This code was used to test my xml output
	//err = ioutil.WriteFile("./output.xml", bite, 0644)
	//check(err)

	//Unmarshal remaining XML, decode from PNG into native image.Image

	err = xml.Unmarshal(bite, &meta)
	check(err)
	near = meta.RDF.Description.Near
	far = meta.RDF.Description.Far
	temp_depth := base64.NewDecoder(base64.StdEncoding, strings.NewReader(meta.RDF.Description.Data))
	depth, err = png.Decode(temp_depth)
	check(err)

	return
}

func MakePointCloud(front_path, back_path, output_path string) {
	//get front and back images, depths, near/far points
	front, front_depth, front_near, front_far := gDepthReader(front_path)
	back, back_depth, back_near, back_far := gDepthReader(back_path)

	//convert images and depth maps to PointCloud
	bounds := front.Bounds()
	point_num := (bounds.Max.Y - bounds.Min.Y) * (bounds.Max.X - bounds.Min.X) * 2
	the_cloud := make([]Point, point_num)
	//Index for pointcloud
	i := 0
	//normalize GDepth near/far by averaging
	near := (front_near + back_near) / 2.0
	far := (front_far + back_far) / 2.0

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			z := findDepth(near, far, front_depth.At(x, y))
			r, g, b, _ := front.At(x, y).RGBA()
			the_cloud[i] = Point{uint(x), uint(y), z, uint(r >> 8), uint(g >> 8), uint(b >> 8)}
			i++
		}
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			z := findDepth(near, far, back_depth.At(x, y))
			r, g, b, _ := back.At(x, y).RGBA()
			the_cloud[i] = Point{uint(bounds.Max.X - x), uint(y), far - z, uint(r >> 8), uint(g >> 8), uint(b >> 8)}
			i++
		}
	}

	file, err := os.Create(output_path)
	check(err)
	defer file.Close()

	writer := bufio.NewWriter(file)
	err = ply_template.Execute(writer, the_cloud)
	check(err)
	writer.Flush()

	return
}

func findDepth(near float64, far float64, d color.Color) (z float64) {
	//For whatever reason this doesn't type assert to color.Gray, so I have to do some stupid math
	r, g, b, _ := d.RGBA()
	//converts to grayscale
	dn := float64((r + g + b) / 3)
	//normalizes once it's a float
	dn /= (255 * 256)
	//use Google Depth map's RangeInverse method to find z-value
	z = (far * near) / (far - dn*(far-near))
	return
}

func main() {
	MakePointCloud("./front.jpg", "./back.jpg", "./the_cloud.ply")
	return
}
