source aliases.sh

sudo docker build -t behlers22/log-aggregator .
sudo docker push behlers22/log-aggregator:latest

sudo kubectl delete -f logpod.yaml
sudo kubectl apply -f logpod.yaml

