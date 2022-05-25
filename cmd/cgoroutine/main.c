#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>

extern void gocallback();

static void* run_thread(void* arg)
{
    for (int i = 0; i < 5; i++) {
        gocallback();
        sleep(1);
    }
    return NULL;
}

int create_threads()
{
    pthread_t threads[50];
    int err;
    for (int i = 0; i < 50; i++) {
        if ((err = pthread_create(&threads[i], NULL, run_thread, NULL)) != 0) {
            return err;
        }
    }
    for (int i = 0; i < 50; i++) {
        pthread_join(threads[i], NULL);
    }
    return 0;
}
