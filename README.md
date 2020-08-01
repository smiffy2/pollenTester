# pollenTester

Sends value transactions to any number of nodes in the network

git clone this repo, go build pollenTester.go and run executable. This should not be run on a goShimmer node.

The program will search the network for the required number of nodes that are in sync and api is open on 8080.

The program will then send transactions between 2 addresses, after confirmed it will send the transaction the other way. It will carry on until it has completed the required number of transactions.

I have run agaisnt 50 nodes hitting each of the 50 five time (instances). 

Running more that 7 instances does seem to cause issues.

Usage of ./pollenTester:
  -collision
        Test collisions
  -inst int
        Number of tests against each node (default 5)
  -nbrNodes int
        Number of nodes you want to test against (default 5)
  -node string
        Valid node for initial transactions (default "http://161.35.154.87:8080")
  -txns int
        Number of transactions to run (default 20)
        
