#   bittorrent-client
A BitTorrent client following the tutorial: https://www.seanjoflynn.com/research/bittorrent.html in Go

#   Motivation
Just use this toy to make myself familiar with Golang :D

#   Concepts
Mainly copied from the tutorial :D

##  Torrent File
A small simple file that contains basic metadata about either a single file or a group of files that are included in the torrent. It specifies how the file should be broken up into pieces as well as which trackers the torrent is being tracked on.

##  Tracker
This is a centralized server that maintains a list of torrents with a corresponding list of peers for each one. The most famous example is The Pirate Bay.

##  Client
The program that can create or open existing torrent files. It connects to the specified trackers and starts either sending or receiving parts of the file as required. Some examples are Vuze, Transmission, uTorrent and Deluge.

#   Bencode
To build a bittorrent client, we first need a Bencode parser. I've noticed several bencode library on the web and I chose [Jackpal's bencode library](https://github.com/jackpal/bencode-go) as the skeleton of our code. Honestly, I don't like its way of decoding the bencode cause it is way too complicated. However, in order to speed things up, let's stay this way.
