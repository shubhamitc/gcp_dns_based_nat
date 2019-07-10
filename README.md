# Intro
This code is designed to help system administrators and cloud administrators to handle cloud nat issue where destination rules are bound with destination IP. However if your custom is running his services behind ElasticBeansTalk then destination IPs are not fixed. 

# Solution 
* Provide a way to update gcp route based on dns resolution
* Make changes in nat-vm using iptables
* Uniform script to do both 
