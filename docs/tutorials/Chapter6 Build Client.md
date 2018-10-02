#   Functionalities
Before we build our own bittorrent client, several things need to be considered. The first and foremost thing of them is what are the functionalities of our torrent client? In general, there are five main functionalities that we need to implement:
1.  Read a torrent file
2.  Request peer lists
3.  Connect to peers
4.  Select a number of leechers that we will allow to request data from us
5.  Select a number of seeders to request data from.

##  Reading Torrents
Our client should be able to handler multiple torrents  simultaneously and hence we should have a mapping data structure inside our client to store all related info about each torrent.
