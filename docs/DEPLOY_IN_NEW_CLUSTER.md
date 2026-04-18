
```sh
# 1. connect to new cluster
export KUBECONFIG=new-cluster.kc
# 2. deploy 99% of app 
make deploy CLUSTER_NAME=mycluster TAG=v1.0.0
# 3. deploy git-token secret 
k apply -f k8s/secret.yaml
# 4. copy webhook certs from hub to new cluster
KUBECONFIG=hub.kc  k -n gitops-reverse-engineer-system get secret gitops-reverse-engineer-certs -o yaml | oc neat > tmp.wh-certs.yaml
# paste it in new cluster
oc apply -f tmp.wh-certs.yaml

# restart the app (scale out/scale in)
oc -n gitops-reverse-engineer-system scale deploy gitops-reverse-engineer --replicas=0
oc -n gitops-reverse-engineer-system scale deploy gitops-reverse-engineer --replicas=1
# Check logs
kubectl logs -n gitops-reverse-engineer-system -l app.kubernetes.io/instance=gitops-reverse-engineer --tail=100
```