import time
from locust import HttpUser, task, between

class HomieUser(HttpUser):
    wait_time = between(1, 2.5)

    @task
    def homie(self):
        self.client.get("/")
