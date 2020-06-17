# ip-whitelisting-route-service-demo-app
A demo app for an IP whitelisting route service in Cloud Foundry

### How to use

```bash
# edit IP whitelist, can contain single IPs or whole subnet ranges
vim ip-whitelist.conf

# push this app
cf push route-service-demo

# create a route service with this app
# https://docs.developer.swisscom.com/services/route-services.html#user-provided
cf create-user-provided-service route-service-demo -r https://ip-whitelisting-demo.scapp.swisscom.com

# check route service if it's there
cf service route-service-demo

# bind the new route service to any of your other apps you want to protect
# https://docs.developer.swisscom.com/devguide/services/route-binding.html
cf bind-route-service scapp.swisscom.com route-service-demo --hostname my-other-app-to-be-protected

# check route if it's there
cf routes | grep my-other-app-to-be-protected
```
