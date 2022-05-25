#include <pthread.h>
#include <stdio.h>
#include <unistd.h>

extern void gocallback();

static void* run_thread(void* arg)
{
    for (;;) {
        gocallback();
        sleep(1);
    }
    return NULL;
}

int create_thread(pthread_t* thread)
{
    return pthread_create(thread, NULL, run_thread, NULL);
}
