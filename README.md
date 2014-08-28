README
==========================

This tool takes two images from Android Lens Blur and tries to make a single pointcloud from them (in PLY format).

A few warnings before you begin:
* Only works on images that are the same size. Don't mix resolutions.
* Makes some assumptions about how the XMP data is encoded and will likely only work on Android Lens Blur photos.
* The resulting PLY files are quite large. Anywhere from 50 to 150 MB.
* Consequently, the tool will take a little while to process your images. Patience, young padawan.

## Usage

Rename your Lens Blur images as "front.jpg" and "back.jpg" then put them in the same directory as "depth-to-pointcloud.go." Then do a little "go run":

	go run depth-to-pointcloud.go

The resulting PYL file be in the same directory and named "the_cloud.ply". Enjoy!
