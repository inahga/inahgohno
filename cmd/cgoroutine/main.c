#include <pthread.h>
#include <stdio.h>
#include <unistd.h>

extern void gocallback();

void* handle_thread(void* arg)
{
    for (;;) {
        gocallback();
        sleep(1);
    }
    return NULL;
}

int run_thread()
{
    int err;
    pthread_t thread;

    if ((err = pthread_create(&thread, NULL, handle_thread, NULL)) != 0) {
        return err;
    }
    printf("thread created\n");
    return 0;
}
